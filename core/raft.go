package core

type Id int64

const InvalidId Id = -1

const All Id = -2

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
}

type Cluster struct {
	Me     Id
	Others []Id
}

func (c * Cluster) majorityCnt() int {
	return len(c.Others) / 2 + 1
}

type Config struct {
	cluster            Cluster
	leader             Id
	electionTimeoutMin int64
	electionTimeoutMax int64
	electionTimeout    int64
	tickCnt            int64
}

type RaftBase struct {
	cfg         Config
	currentTerm Term
	votedFor    Id
	commitIndex Index
	lastApplied Index
	log         []Entry
}

func newRaftBase(cfg Config) RaftBase {
	return RaftBase{
		cfg:         cfg,
		currentTerm: InvalidTerm,
		votedFor:    InvalidId,
		commitIndex: InvalidIndex,
		lastApplied: InvalidIndex,
		log:         make([]Entry, 0),
	}
}

func (r *RaftBase) moveState(to RaftObject) Msg {
	return Msg{
		tp:      MoveState,
		payload: to,
	}
}

func (r *RaftBase) Resp(to Id, payload interface{}) Msg {
	return Msg{
		tp:      Resp,
		from:    r.cfg.cluster.Me,
		to:      to,
		payload: payload,
	}
}

func (r *RaftBase) pointReq(dest Id, payload interface{}) Msg {
	return Msg{
		tp:      Req,
		from:    r.cfg.cluster.Me,
		to:      dest,
		payload: payload,
	}
}

func (r *RaftBase) broadcastReq(payload interface{}) Msg {
	return r.pointReq(All, payload)
}
