package core

type Id int64
const InvalidId Id = -1

type Term int64
type Index int64
type Command string

type Entry struct {
	term Term
	cmd Command
}

type RaftObject interface {
	TakeAction(req interface{}) (obj RaftObject, resp interface{})
}

type Cluster struct {
	Me Id
	Others []Id
}

type RaftBase struct {
	cluster Cluster
	currentTerm Term
	votedFor Id
	commitIndex Index
	lastApplied Index
	log []Entry
}

func newRaftBase(cluster Cluster) RaftBase {
	return RaftBase {
		cluster: cluster,
		currentTerm: 0,
		votedFor: InvalidId,
		commitIndex: 0,
		lastApplied: 0,
		log: make([]Entry, 0),
	}
}

type Follower RaftBase

func NewFollower(cluster Cluster) *Follower {
	follower := Follower(newRaftBase(cluster))
	return &follower
}

func (f *Follower) TakeAction(req interface{}) (obj RaftObject, resp interface{}) {
	switch req.(type) {
	case *RequestVoteReq:
		return f, f.vote(req.(*RequestVoteReq))
	case *AppendEntriesReq:
		return nil, nil
	default:
		panic("Shouldn't goes here")
	}
}

func (f *Follower) vote(req *RequestVoteReq) *RequestVoteResp {
	if req.Term < f.currentTerm {
		return &RequestVoteResp{
			VoteGranted: false,
			Term: f.currentTerm,
		}
	}

	f.currentTerm = req.Term
	f.votedFor = req.CandidateId

	return &RequestVoteResp{
		VoteGranted: true,
		Term: f.currentTerm,
	}
}

type Candidate RaftBase

type Leader struct {
	base RaftBase
	nextIndex []Index
	matchIndex []Index
}

