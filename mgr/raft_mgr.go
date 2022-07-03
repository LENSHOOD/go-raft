package mgr

import (
	"context"
	. "github.com/LENSHOOD/go-raft/comm"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/opentracing/opentracing-go"
	opLog "github.com/opentracing/opentracing-go/log"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

type Rpc struct {
	Ctx     context.Context
	Addr    core.Address
	Payload interface{}
}

type ConfigOp int

const (
	Add ConfigOp = iota
	Remove
)

type ConfigChange struct {
	Op     ConfigOp
	Server core.Address
}

type Config struct {
	TickIntervalMilliSec int64
	ElectionTimeoutMin   int64
	ElectionTimeoutMax   int64
	DebugMode            bool
}

type switcher struct {
	state uint64
	wg    sync.WaitGroup
}

func (s *switcher) on() bool {
	if atomic.LoadUint64(&s.state) != 0 {
		return false
	}

	s.wg.Wait()
	s.wg.Add(1)
	atomic.StoreUint64(&s.state, 1)
	return true
}

func (s *switcher) off() {
	if atomic.LoadUint64(&s.state) != 0 {
		atomic.StoreUint64(&s.state, 0)
		s.wg.Wait()
	}
}

func (s *switcher) isOff() bool {
	if atomic.LoadUint64(&s.state) != 0 {
		return false
	}

	s.wg.Done()
	return true
}

type dispatcher struct {
	reqOutput   chan<- *Rpc
	respOutputs sync.Map
}

func (d *dispatcher) RegisterReq(ch chan<- *Rpc) {
	d.reqOutput = ch
}

func (d *dispatcher) RegisterResp(addr core.Address) <-chan *Rpc {
	ch := make(chan *Rpc)
	ret, loaded := d.respOutputs.LoadOrStore(addr, ch)
	if loaded {
		close(ch)
	}
	return ret.(chan *Rpc)
}

func (d *dispatcher) Cancel(addr core.Address) {
	ch, loaded := d.respOutputs.LoadAndDelete(addr)
	if loaded {
		close(ch.(chan *Rpc))
	}
}

func (d *dispatcher) dispatch(rpc *Rpc) {
	defer func() {
		if err := recover(); err != nil {
			GetLogger().Errorf("[MGR] failed to dispatch due to panic: %v", err)
		}
	}()

	switch rpc.Payload.(type) {
	case *core.AppendEntriesReq, *core.RequestVoteReq, *core.TimeoutNowReq:
		if d.reqOutput != nil {
			d.reqOutput <- rpc
		} else {
			GetLogger().Errorf("[MGR] request channel haven't registered yet, dispatch failed...")
		}
	case *core.AppendEntriesResp, *core.RequestVoteResp, *core.CmdResp:
		if ch, exist := d.respOutputs.Load(rpc.Addr); exist {
			ch.(chan *Rpc) <- rpc
		} else {
			GetLogger().Errorf("[MGR] response channel not found: %s, dispatch failed...", rpc.Addr)
		}
	}
}

func (d *dispatcher) clearAll() {
	d.respOutputs.Range(func(_, v interface{}) bool {
		close(v.(chan *Rpc))
		return true
	})

	d.respOutputs = sync.Map{}
	d.reqOutput = nil
}

type Ticker interface {
	GetTickCh() <-chan time.Time
	Start()
	Stop()
}

type defaultTicker struct {
	d      time.Duration
	ticker *time.Ticker
}

func (dt *defaultTicker) GetTickCh() <-chan time.Time {
	return dt.ticker.C
}

func (dt *defaultTicker) Start() {
	dt.ticker.Reset(dt.d)
}

func (dt *defaultTicker) Stop() {
	dt.ticker.Stop()
}

func NewDefaultTicker(d time.Duration) *defaultTicker {
	ticker := time.NewTicker(d)
	ticker.Stop()

	return &defaultTicker{
		d:      d,
		ticker: ticker,
	}
}

type RaftManager struct {
	obj        core.RaftObject
	input      chan *Rpc
	ticker     Ticker
	cfg        Config
	switcher   switcher
	Dispatcher dispatcher
}

func (m *RaftManager) Run() {
	if !m.switcher.on() {
		// should only run once
		return
	}

	m.ticker.Start()
	GetLogger().Infof("[MGR-%s] Raft Manager Started.", m.me())

	for !m.switcher.isOff() {
		ctx := context.Background()
		res := core.NullMsg
		select {
		case _ = <-m.ticker.GetTickCh():
			res = m.obj.(core.RaftObject).TakeAction(core.Msg{Tp: core.Tick})
			if res != core.NullMsg {
				span, spctx := opentracing.StartSpanFromContext(ctx, "mgr-meet-interval")
				span.SetTag("raft-obj", reflect.TypeOf(m.obj))
				ctx = spctx
				span.Finish()
			}
		case req := <-m.input:
			span, spctx := opentracing.StartSpanFromContext(req.Ctx, "mgr-received-rpc")
			ctx = spctx

			GetLogger().Infof("[MGR-%s] Received from %s: %s", m.me(), req.Addr, req.Payload)

			if cc, ok := req.Payload.(*ConfigChange); ok {
				req.Payload = &core.CmdReq{Cmd: m.convertConfigChangeToCmd(cc)}
			}

			tp := core.Rpc
			if _, ok := req.Payload.(*core.CmdReq); ok {
				tp = core.Cmd
			}
			span.SetTag("rpc-tp", tp)

			res = m.obj.TakeAction(core.Msg{
				Tp:      tp,
				From:    req.Addr,
				To:      m.me(),
				Payload: req.Payload,
			})
			span.Finish()
		}

		if res != core.NullMsg {
			// once go into this branch, there will be a whole new round of req-resp loop started,
			// ctx should be replaced to a new one in case of the previous ctx be canceled
			newCtx := opentracing.ContextWithSpan(context.Background(), opentracing.SpanFromContext(ctx))
			span, spctx := opentracing.StartSpanFromContext(newCtx, "mgr-process-raft-result")
			switch res.Tp {
			case core.MoveState:
				span.LogFields(opLog.Object("move-state", res.Payload))
				m.obj = res.Payload.(core.RaftObject)
				GetLogger().Infof("[MGR-%s] Role Changed: %T", m.me(), res.Payload)
			case core.Rpc:
				if resp, ok := res.Payload.(*core.CmdResp); ok && !resp.Success {
					// if no leader elected yet, return self address to let client give another try
					if leaderId, ok := resp.Result.(core.Address); ok && leaderId == core.InvalidId {
						resp.Result = m.me()
					}
				}

				span.LogFields(opLog.Object("rpc-respond", res.Payload))
				go m.sendTo(spctx, res.To, res.Payload)
			}
			span.Finish()
		}
	}
}

func (m *RaftManager) sendTo(ctx context.Context, to core.Address, payload interface{}) {
	dispatch := func(rpc *Rpc) {
		if rpc == nil {
			return
		}

		m.Dispatcher.dispatch(rpc)

		if _, ok := payload.(*core.CmdResp); ok {
			m.Dispatcher.Cancel(rpc.Addr)
		}

		GetLogger().Infof("[MGR-%s] Sent: [%s], msg: %s", m.me(), rpc.Addr, payload)
	}

	switch to {
	case core.All:
		for _, addr := range m.others() {
			dispatch(&Rpc{ctx, addr, payload})
		}
	case core.Composed:
		for _, msg := range payload.([]core.Msg) {
			dispatch(&Rpc{ctx, msg.To, msg.Payload})
		}
	default:
		dispatch(&Rpc{ctx, to, payload})
	}
}

func (m *RaftManager) Stop() {
	m.switcher.off()
	m.Dispatcher.clearAll()
}

func (m *RaftManager) me() core.Address {
	return m.obj.GetCluster().Me
}

func (m *RaftManager) others() []core.Address {
	all := m.obj.GetCluster().Members
	if len(all) == 0 {
		return make([]core.Address, 0)
	}

	others := make([]core.Address, 0, len(all)-1)
	for _, addr := range all {
		if addr == m.me() {
			continue
		}
		others = append(others, addr)
	}

	return others
}

func (m *RaftManager) convertConfigChangeToCmd(cc *ConfigChange) core.Command {
	currMember := m.obj.GetCluster().Members
	newMember := make([]core.Address, 0)
	givenSvr := cc.Server

	switch cc.Op {
	case Add:
		newMember = append(append(newMember, currMember...), givenSvr)
	case Remove:
		for _, addr := range currMember {
			if addr == givenSvr {
				continue
			}

			newMember = append(newMember, addr)
		}
	}

	return &core.ConfigChangeCmd{Members: newMember, PrevMembers: currMember}
}

func (m *RaftManager) assertDebugMode() {
	if !m.cfg.DebugMode {
		GetLogger().Fatalf("Cannot run debug method if DebugMode is off.")
	}
}

func (m *RaftManager) IsLeader() bool {
	m.assertDebugMode()
	_, ok := m.obj.(*core.Leader)
	return ok
}

func (m *RaftManager) IsCandidate() bool {
	m.assertDebugMode()
	_, ok := m.obj.(*core.Candidate)
	return ok
}

func (m *RaftManager) IsFollower() bool {
	m.assertDebugMode()
	_, ok := m.obj.(*core.Follower)
	return ok
}

func (m *RaftManager) GetAllEntries() []core.Entry {
	m.assertDebugMode()
	return m.obj.GetAllEntries()
}

func (m *RaftManager) GetOthers() []core.Address {
	m.assertDebugMode()
	return m.others()
}

func NewRaftMgr(cls core.Cluster, cfg Config, sm core.StateMachine, inputCh chan *Rpc) *RaftManager {
	return NewRaftMgrWithTicker(cls, cfg, sm, inputCh, NewDefaultTicker(time.Millisecond*time.Duration(cfg.TickIntervalMilliSec)))
}

func NewRaftMgrWithTicker(cls core.Cluster, cfg Config, sm core.StateMachine, inputCh chan *Rpc, ticker Ticker) *RaftManager {
	mgr := RaftManager{
		input: inputCh,
		cfg:   cfg,
	}

	if ticker == nil {
		GetLogger().Fatalf("Ticker should be provided.")
	}
	mgr.ticker = ticker

	// as always, follower at beginning
	mgr.obj = core.NewFollower(core.InitConfig(cls, cfg.ElectionTimeoutMin, cfg.ElectionTimeoutMax), sm)

	return &mgr
}
