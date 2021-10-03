package go_raft

import (
	"go-raft/core"
	"hash/maphash"
)

type Address string
type Rpc struct {
	Addr Address
	Payload interface{}
}

type Config struct {
	me Address
	others               []Address
	tickIntervalMilliSec int
	electionTimeoutMin   int64
	electionTimeoutMax int64
}

type RaftManager struct {
	obj core.RaftObject
	input chan *Rpc
	output chan *Rpc
	cfg       Config
	addrMapId map[Address]core.Id
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
		Me: genId(cfg.me),
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