package go_raft

import (
	"fmt"
	"go-raft/core"
	"hash/maphash"
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

type RaftManager struct {
	obj    core.RaftObject
	input  chan *Rpc
	output chan *Rpc
	ticker *time.Ticker
	cfg    Config
	addrIdMapper
}

func (m *RaftManager) Run() {
	m.ticker = time.NewTicker(time.Millisecond * time.Duration(m.cfg.tickIntervalMilliSec))

	res := core.NullMsg
	select {
	case _ = <-m.ticker.C:
		res = m.obj.TakeAction(core.Msg{Tp: core.Tick})
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
