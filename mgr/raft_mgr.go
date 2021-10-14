package mgr

import (
	"github.com/LENSHOOD/go-raft/core"
	"hash/fnv"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var logger = log.Default()

type Address string
type Rpc struct {
	Addr    Address
	Payload interface{}
}

type Config struct {
	Me                   Address
	Others               []Address
	TickIntervalMilliSec int64
	ElectionTimeoutMin   int64
	ElectionTimeoutMax   int64
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
	switch rpc.Payload.(type) {
	case *core.AppendEntriesReq, *core.RequestVoteReq:
		if d.reqOutput != nil {
			d.reqOutput <- rpc
		} else {
			logger.Fatalf("[MGR] request channel haven't registered yet, dispatch failed...")
		}
	case *core.AppendEntriesResp, *core.RequestVoteResp, *core.CmdResp:
		if ch, exist := d.respOutputs.Load(rpc.Addr); exist {
			ch.(chan *Rpc) <- rpc
		} else {
			logger.Fatalf("[MGR] response channel not found: %s, dispatch failed...", rpc.Addr)
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
	d time.Duration
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
		d: d,
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
		res := core.NullMsg
		select {
		case _ = <-m.ticker.GetTickCh():
			res = m.obj.(core.RaftObject).TakeAction(core.Msg{Tp: core.Tick})
		case req := <-m.input:
			logger.Printf("[MGR-%s] Received from %s: %s", m.cfg.Me, req.Addr, req.Payload)
			tp := core.Rpc
			if _, ok := req.Payload.(*core.CmdReq); ok {
				tp = core.Cmd
			}

			res = m.obj.TakeAction(core.Msg{
				Tp:      tp,
				From:    m.getIdByAddr(req.Addr),
				To:      m.getIdByAddr(m.cfg.Me),
				Payload: req.Payload,
			})
		}

		if res != core.NullMsg {
			switch res.Tp {
			case core.MoveState:
				m.obj = res.Payload.(core.RaftObject)
				logger.Printf("[MGR-%s] Role Changed: %T", m.cfg.Me, res.Payload)
			case core.Rpc:
				_ = m.sendTo(res.To, res.Payload)
			}
		}
	}
}

func (m *RaftManager) sendTo(to core.Id, payload interface{}) error {
	buildRpc := func(to core.Id, payload interface{}) *Rpc {
		if addr, exist := m.getAddrById(to); exist {
			return &Rpc{addr, payload}
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
			dispatch(&Rpc{addr, payload})
		}
	case core.Composed:
		for _, msg := range payload.([]core.Msg) {
			dispatch(buildRpc(msg.To, msg.Payload))
		}
	default:
		dispatch(buildRpc(to, payload))
	}

	return nil
}

func (m *RaftManager) Stop() {
	m.switcher.off()
	m.Dispatcher.clearAll()
}

func (m *RaftManager) IsLeader() bool {
	_, ok := m.obj.(*core.Leader)
	return ok
}

func NewRaftMgr(cfg Config, sm core.StateMachine, inputCh chan *Rpc) *RaftManager {
	return NewRaftMgrWithTicker(cfg, sm, inputCh, NewDefaultTicker(time.Millisecond * time.Duration(cfg.TickIntervalMilliSec)))
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
	for _, addr := range cfg.Others {
		cls.Others = append(cls.Others, mgr.getIdByAddr(addr))
	}

	if ticker == nil {
		log.Fatalf("Ticker should be provided.")
	}
	mgr.ticker = ticker

	// as always, follower at beginning
	mgr.obj = core.NewFollower(core.InitConfig(cls, cfg.ElectionTimeoutMin, cfg.ElectionTimeoutMax), sm)

	return &mgr
}

var hash = fnv.New64()

func genId(addr Address) core.Id {
	_, _ = hash.Write([]byte(addr))
	return core.Id(hash.Sum64())
}
