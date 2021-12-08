package mgr

import (
	"context"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/opentracing/opentracing-go"
	opLog "github.com/opentracing/opentracing-go/log"
	"hash/fnv"
	"log"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

var logger = log.Default()

type Address string
type Rpc struct {
	Ctx     context.Context
	Addr    Address
	Payload interface{}
}

type ConfigOp int

const (
	Add ConfigOp = iota
	Remove
)

type ConfigChange struct {
	Op     ConfigOp
	Server Address
}

type Config struct {
	Me                   Address
	Others               []Address
	TickIntervalMilliSec int64
	ElectionTimeoutMin   int64
	ElectionTimeoutMax   int64
	DebugMode            bool
}

type addrIdMapper struct {
	addrMapId sync.Map
	idMapAddr sync.Map
}

func (aim *addrIdMapper) getIdByAddr(addr Address) core.Id {
	id, loaded := aim.addrMapId.LoadOrStore(addr, genId(addr))
	if !loaded {
		aim.idMapAddr.Store(id, addr)
	}

	return id.(core.Id)
}

func (aim *addrIdMapper) getAddrById(id core.Id) (Address, bool) {
	if addr, ok := aim.idMapAddr.Load(id); ok {
		return addr.(Address), ok
	}

	return *new(Address), false
}

func (aim *addrIdMapper) remove(addr Address) {
	id, loaded := aim.addrMapId.LoadAndDelete(addr)
	if loaded {
		aim.idMapAddr.Delete(id)
	}
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

func (d *dispatcher) RegisterResp(addr Address) <-chan *Rpc {
	ch := make(chan *Rpc)
	ret, loaded := d.respOutputs.LoadOrStore(addr, ch)
	if loaded {
		close(ch)
	}
	return ret.(chan *Rpc)
}

func (d *dispatcher) Cancel(addr Address) {
	ch, loaded := d.respOutputs.LoadAndDelete(addr)
	if loaded {
		close(ch.(chan *Rpc))
	}
}

func (d *dispatcher) dispatch(rpc *Rpc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Printf("[MGR] failed to dispatch due to panic: %v", err)
		}
	}()

	switch rpc.Payload.(type) {
	case *core.AppendEntriesReq, *core.RequestVoteReq:
		if d.reqOutput != nil {
			d.reqOutput <- rpc
		} else {
			logger.Printf("[MGR] request channel haven't registered yet, dispatch failed...")
		}
	case *core.AppendEntriesResp, *core.RequestVoteResp, *core.CmdResp:
		if ch, exist := d.respOutputs.Load(rpc.Addr); exist {
			ch.(chan *Rpc) <- rpc
		} else {
			logger.Printf("[MGR] response channel not found: %s, dispatch failed...", rpc.Addr)
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
	obj    core.RaftObject
	input  chan *Rpc
	ticker Ticker
	cfg    Config
	addrIdMapper
	switcher   switcher
	Dispatcher dispatcher
}

func (m *RaftManager) Run() {
	if !m.switcher.on() {
		// should only run once
		return
	}

	m.ticker.Start()
	logger.Printf("[MGR-%s] Raft Manager Started.", m.cfg.Me)

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

			logger.Printf("[MGR-%s] Received from %s: %s", m.cfg.Me, req.Addr, req.Payload)

			if cc, ok := res.Payload.(*ConfigChange); ok {
				res.Payload = &core.CmdReq{Cmd: m.convertConfigChangeToCmd(cc)}
			}

			tp := core.Rpc
			if _, ok := req.Payload.(*core.CmdReq); ok {
				tp = core.Cmd
			}
			span.SetTag("rpc-tp", tp)

			res = m.obj.TakeAction(core.Msg{
				Tp:      tp,
				From:    m.getIdByAddr(req.Addr),
				To:      m.getIdByAddr(m.cfg.Me),
				Payload: req.Payload,
			})
			span.Finish()
		}

		if res != core.NullMsg {
			// update whole cluster to catch up with raft object config change
			// TODO: only a temporary solution, try more elegant approach
			m.updateCluster(m.obj.GetCluster())

			span, spctx := opentracing.StartSpanFromContext(ctx, "mgr-process-raft-result")
			switch res.Tp {
			case core.MoveState:
				span.LogFields(opLog.Object("move-state", res.Payload))
				m.obj = res.Payload.(core.RaftObject)
				logger.Printf("[MGR-%s] Role Changed: %T", m.cfg.Me, res.Payload)
			case core.Rpc:
				if resp, ok := res.Payload.(*core.CmdResp); ok && !resp.Success {
					leaderId, _ := resp.Result.(core.Id)

					// if no leader elected yet, return self address to let client give another try
					addr := m.cfg.Me
					if leaderAddress, exist := m.getAddrById(leaderId); exist {
						addr = leaderAddress
					}

					resp.Result = addr
				}

				span.LogFields(opLog.Object("rpc-respond", res.Payload))
				go m.sendTo(spctx, res.To, res.Payload)
			}
			span.Finish()
		}
	}
}

func (m *RaftManager) sendTo(ctx context.Context, to core.Id, payload interface{}) {
	buildRpc := func(ctx context.Context, to core.Id, payload interface{}) *Rpc {
		if addr, exist := m.getAddrById(to); exist {
			return &Rpc{ctx, addr, payload}
		}

		logger.Fatalf("dest not exist, id: %d", to)
		return nil
	}

	dispatch := func(rpc *Rpc) {
		if rpc == nil {
			return
		}

		m.Dispatcher.dispatch(rpc)

		if _, ok := payload.(*core.CmdResp); ok {
			m.remove(rpc.Addr)
			m.Dispatcher.Cancel(rpc.Addr)
		}

		logger.Printf("[MGR-%s] Sent: [%s], msg: %s", m.cfg.Me, rpc.Addr, payload)
	}

	switch to {
	case core.All:
		for _, addr := range m.cfg.Others {
			dispatch(&Rpc{ctx, addr, payload})
		}
	case core.Composed:
		for _, msg := range payload.([]core.Msg) {
			dispatch(buildRpc(ctx, msg.To, msg.Payload))
		}
	default:
		dispatch(buildRpc(ctx, to, payload))
	}
}

func (m *RaftManager) Stop() {
	m.switcher.off()
	m.Dispatcher.clearAll()
}

func (m *RaftManager) updateCluster(cls core.Cluster) {
	var others []Address
	for _, id := range cls.Members {
		addr, exist := m.getAddrById(id)
		if !exist {
			log.Fatalf("Cluster server not found, id %d.", id)
		}
		if addr != m.cfg.Me {
			others = append(others, addr)
		}
	}

	m.cfg.Others = others
}

func (m *RaftManager) convertConfigChangeToCmd(cc *ConfigChange) core.Command {
	currMember := m.obj.GetCluster().Members
	newMember := make([]core.Id, 0)
	givenId := m.getIdByAddr(cc.Server)

	switch cc.Op {
	case Add:
		newMember = append(append(newMember, currMember...), givenId)
	case Remove:
		for _, id := range currMember {
			if id == givenId {
				continue
			}

			newMember = append(newMember, id)
		}
	}

	return &core.ConfigChangeCmd{Members: newMember, PrevMembers: currMember}
}

func (m *RaftManager) assertDebugMode() {
	if !m.cfg.DebugMode {
		log.Fatalf("Cannot run debug method if DebugMode is off.")
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

func NewRaftMgr(cfg Config, sm core.StateMachine, inputCh chan *Rpc) *RaftManager {
	return NewRaftMgrWithTicker(cfg, sm, inputCh, NewDefaultTicker(time.Millisecond*time.Duration(cfg.TickIntervalMilliSec)))
}

func NewRaftMgrWithTicker(cfg Config, sm core.StateMachine, inputCh chan *Rpc, ticker Ticker) *RaftManager {
	mgr := RaftManager{
		input: inputCh,
		cfg:   cfg,
	}

	// build cluster with id
	cls := core.Cluster{
		Me: mgr.getIdByAddr(cfg.Me),
	}
	cls.Members = append(cls.Members, cls.Me)
	for _, addr := range cfg.Others {
		cls.Members = append(cls.Members, mgr.getIdByAddr(addr))
	}

	if ticker == nil {
		log.Fatalf("Ticker should be provided.")
	}
	mgr.ticker = ticker

	// as always, follower at beginning
	mgr.obj = core.NewFollower(core.InitConfig(cls, cfg.ElectionTimeoutMin, cfg.ElectionTimeoutMax), sm)

	return &mgr
}

func genId(addr Address) core.Id {
	hash := fnv.New64()
	_, _ = hash.Write([]byte(addr))
	return core.Id(hash.Sum64())
}
