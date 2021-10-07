package go_raft

import (
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

type RaftManager struct {
	obj       core.RaftObject
	input     chan *Rpc
	output    chan *Rpc
	ticker    *time.Ticker
	cfg       Config
	addrMapId map[Address]core.Id
}

func (m *RaftManager) Run() {
	m.ticker = time.NewTicker(time.Millisecond * time.Duration(m.cfg.tickIntervalMilliSec))

	res := core.NullMsg
	select {
	case _ = <-m.ticker.C:
		res = m.obj.TakeAction(core.Msg{Tp: core.Tick})
	case req := <-m.input:
		fromId, isExist := m.addrMapId[req.Addr]
		if !isExist {
			fromId = genId(req.Addr)
			m.addrMapId[req.Addr] = fromId
		}

		res = m.obj.TakeAction(core.Msg{
			Tp:      core.Rpc,
			From:    fromId,
			To:      m.addrMapId[m.cfg.me],
			Payload: req.Payload,
		})
	}

	if res != core.NullMsg {
		switch res.Tp {
		case core.MoveState:
			m.obj = res.Payload.(core.RaftObject)
		case core.Rpc:
			if res.To == core.All {
				for _, addr := range m.cfg.others {
					m.output <- &Rpc{
						Addr:    addr,
						Payload: res.Payload,
					}
				}
			} else {
				m.sendTo(res.To, res.Payload)
			}
		}
	}
}

func (m *RaftManager) sendTo(to core.Id, payload interface{}) {
	for k, v := range m.addrMapId {
		if v == to {
			m.output <- &Rpc{
				Addr:    k,
				Payload: payload,
			}

			break
		}
	}
}

func NewRaftMgr(cfg Config, sm core.StateMachine, inputCh chan *Rpc, outputCh chan *Rpc) *RaftManager {
	mgr := RaftManager{
		input:     inputCh,
		output:    outputCh,
		cfg:       cfg,
		addrMapId: make(map[Address]core.Id),
	}

	// build cluster with id
	cls := core.Cluster{
		Me:     genId(cfg.me),
		Others: []core.Id{},
	}
	mgr.addrMapId[cfg.me] = cls.Me

	for _, v := range cfg.others {
		id := genId(v)
		cls.Others = append(cls.Others, id)
		mgr.addrMapId[v] = id
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
