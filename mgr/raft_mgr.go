package mgr

import (
	"fmt"
	"github.com/LENSHOOD/go-raft/core"
	"hash/maphash"
	"sync"
	"sync/atomic"
	"time"
)

type Address string
type Rpc struct {
	Addr    Address
	Payload interface{}
}

type Config struct {
	me                   Address
	others               []Address
	tickIntervalMilliSec int64
	electionTimeoutMin   int64
	electionTimeoutMax   int64
}

type addrIdMapper struct {
	addrMapId map[Address]core.Id
	idMapAddr map[core.Id]Address
}

func (aim *addrIdMapper) getIdByAddr(addr Address) core.Id {
	if id, exist := aim.addrMapId[addr]; exist {
		return id
	}

	id := genId(addr)
	aim.addrMapId[addr] = id
	aim.idMapAddr[id] = addr
	return id
}

func (aim *addrIdMapper) getAddrById(id core.Id) (Address, bool) {
	addr, exist := aim.idMapAddr[id]
	return addr, exist
}

func (aim *addrIdMapper) remove(addr Address) {
	if id, exist := aim.addrMapId[addr]; exist {
		delete(aim.idMapAddr, id)
		delete(aim.addrMapId, addr)
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
	reqOutput chan<- *Rpc
	respOutputs map[Address]chan *Rpc
}

func (d *dispatcher) RegisterReq(ch chan<- *Rpc) {
	d.reqOutput = ch
}

func (d *dispatcher) RegisterResp(addr Address) <-chan *Rpc {
	if ch, exist := d.respOutputs[addr]; exist {
		return ch
	}

	ch := make(chan *Rpc)
	d.respOutputs[addr] = ch
	return ch
}

func (d *dispatcher) Cancel(addr Address) {
	if ch, exist := d.respOutputs[addr]; exist {
		close(ch)
		delete(d.respOutputs, addr)
	}
}

func (d *dispatcher) dispatch(rpc *Rpc) {
	switch rpc.Payload.(type) {
	case *core.AppendEntriesReq, *core.RequestVoteReq:
		if d.reqOutput != nil {
			d.reqOutput <- rpc
		}
	case *core.AppendEntriesResp, *core.RequestVoteResp, *core.CmdResp:
		if ch, exist := d.respOutputs[rpc.Addr]; exist {
			ch <- rpc
		}
	}
}

func (d *dispatcher) clearAll() {
	for addr := range d.respOutputs {
		d.Cancel(addr)
	}

	d.reqOutput = nil
}

type RaftManager struct {
	obj    core.RaftObject
	input  chan *Rpc
	output chan *Rpc
	ticker *time.Ticker
	cfg    Config
	addrIdMapper
	switcher switcher
}

func (m *RaftManager) Run() {
	if !m.switcher.on() {
		// should only run once
		return
	}
	m.ticker = time.NewTicker(time.Millisecond * time.Duration(m.cfg.tickIntervalMilliSec))

	for !m.switcher.isOff() {
		res := core.NullMsg
		select {
		case _ = <-m.ticker.C:
			res = m.obj.(core.RaftObject).TakeAction(core.Msg{Tp: core.Tick})
		case req := <-m.input:
			tp := core.Rpc
			if _, ok := req.Payload.(*core.CmdReq); ok {
				tp = core.Cmd
			}

			res = m.obj.TakeAction(core.Msg{
				Tp:      tp,
				From:    m.getIdByAddr(req.Addr),
				To:      m.getIdByAddr(m.cfg.me),
				Payload: req.Payload,
			})
		}

		if res != core.NullMsg {
			switch res.Tp {
			case core.MoveState:
				m.obj = res.Payload.(core.RaftObject)
			case core.Rpc:
				_ = m.sendTo(res.To, res.Payload)
			}
		}
	}
}

func (m *RaftManager) sendTo(to core.Id, payload interface{}) error {
	var addrs []Address
	if to == core.All {
		addrs = append(addrs, m.cfg.others...)
	} else if addr, exist := m.getAddrById(to); exist {
		addrs = append(addrs, addr)
	} else {
		return fmt.Errorf("dest not exist, id: %d", to)
	}

	for _, addr := range addrs {
		m.output <- &Rpc{
			Addr:    addr,
			Payload: payload,
		}

		if _, ok := payload.(*core.CmdResp); ok {
			m.remove(addr)
		}
	}

	return nil
}

func (m *RaftManager) Stop() {
	m.switcher.off()
	// TODO: close all remain dispatcher channels
}

func NewRaftMgr(cfg Config, sm core.StateMachine, inputCh chan *Rpc, outputCh chan *Rpc) *RaftManager {
	mgr := RaftManager{
		input:  inputCh,
		output: outputCh,
		cfg:    cfg,
		addrIdMapper: addrIdMapper{
			idMapAddr: make(map[core.Id]Address),
			addrMapId: make(map[Address]core.Id),
		},
	}

	// build cluster with id
	cls := core.Cluster{
		Me: mgr.getIdByAddr(cfg.me),
	}
	for _, addr := range cfg.others {
		cls.Others = append(cls.Others, mgr.getIdByAddr(addr))
	}

	// as always, follower at beginning
	mgr.obj = core.NewFollower(core.InitConfig(cls, cfg.electionTimeoutMin, cfg.electionTimeoutMax), sm)

	return &mgr
}

var hash maphash.Hash

func genId(addr Address) core.Id {
	_, _ = hash.WriteString(string(addr))
	return core.Id(hash.Sum64())
}
