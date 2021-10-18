package core

import (
	"math/rand"
)

type Id int64

const InvalidId Id = -1

const All Id = -2

const Composed Id = -3

// InvalidTerm is the init value of term, which will become 1 after first follower turn to candidate.
const InvalidTerm Term = 0

// InvalidIndex is the init value of any indexes. First valid index is 1.
const InvalidIndex Index = 0

type Term int64
type Index int64
type Command string

type Entry struct {
	Term Term
	Idx  Index
	Cmd  Command
}

type RaftObject interface {
	TakeAction(msg Msg) Msg
	GetAllEntries() []Entry
}

type Cluster struct {
	Me     Id
	Others []Id
}

func (c *Cluster) majorityCnt() int {
	return len(c.Others)/2 + 1
}

type Config struct {
	cluster            Cluster
	leader             Id
	electionTimeoutMin int64
	electionTimeoutMax int64
	electionTimeout    int64
	tickCnt            int64
}

func InitConfig(cls Cluster, eleTimeoutMin int64, eleTimeoutMax int64) Config {
	return Config{
		cluster:            cls,
		leader:             InvalidId,
		electionTimeoutMin: eleTimeoutMin,
		electionTimeoutMax: eleTimeoutMax,
		electionTimeout:    rand.Int63n(eleTimeoutMax-eleTimeoutMin) + eleTimeoutMin,
		tickCnt:            0,
	}
}

type RaftBase struct {
	cfg         Config
	currentTerm Term
	votedFor    Id
	commitIndex Index
	lastApplied Index
	log         []Entry
	sm          StateMachine
}

func newRaftBase(cfg Config, sm StateMachine) RaftBase {
	return RaftBase{
		cfg:         cfg,
		currentTerm: InvalidTerm,
		votedFor:    InvalidId,
		commitIndex: InvalidIndex,
		lastApplied: InvalidIndex,
		log:         make([]Entry, 0),
		sm:          sm,
	}
}

func (r *RaftBase) moveState(to RaftObject) Msg {
	return Msg{
		Tp:      MoveState,
		Payload: to,
	}
}

func (r *RaftBase) Resp(to Id, payload interface{}) Msg {
	return Msg{
		Tp:      Rpc,
		From:    r.cfg.cluster.Me,
		To:      to,
		Payload: payload,
	}
}

func (r *RaftBase) pointReq(dest Id, payload interface{}) Msg {
	return Msg{
		Tp:      Rpc,
		From:    r.cfg.cluster.Me,
		To:      dest,
		Payload: payload,
	}
}

func (r *RaftBase) broadcastReq(payload interface{}) Msg {
	return r.pointReq(All, payload)
}

func (r *RaftBase) composedReq(toSet []Id, payloadSupplier func(to Id) interface{}) Msg {
	var msgs []Msg
	for _, to := range toSet {
		msgs = append(msgs, r.pointReq(to, payloadSupplier(to)))
	}

	return r.pointReq(Composed, msgs)
}

var InvalidEntry = Entry{
	Term: InvalidTerm,
	Idx:  InvalidIndex,
	Cmd:  "",
}

func (r *RaftBase) getLastEntry() Entry {
	if len(r.log) > 0 {
		return r.log[len(r.log)-1]
	}

	return InvalidEntry
}

func (r *RaftBase) getEntryByIdx(idx Index) Entry {
	for _, e := range r.log {
		if e.Idx == idx {
			return e
		}
	}

	return InvalidEntry
}

func (r *RaftBase) applyCmdToStateMachine() interface{} {
	entry := r.getEntryByIdx(r.commitIndex)
	if entry == InvalidEntry {
		panic("cannot find entry by idx")
	}

	res := r.sm.Exec(entry.Cmd)
	r.lastApplied = r.commitIndex
	return res
}

func (r *RaftBase) GetAllEntries() []Entry {
	return r.log
}