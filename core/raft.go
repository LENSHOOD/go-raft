package core

type Id int64
const InvalidId Id = -1

type Term int64
type Index int64
type Command string

type Entry struct {
	Term Term
	Idx Index
	Cmd Command
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

// vote for some candidate, rules:
//     1. if term < currentTerm, not vote
//     2. first-come-first-served, if already vote to candidate-a, then not vote to candidate-b
//     3. if follower's last log entry's term or index bigger than candidate, not vote
func (f *Follower) vote(req *RequestVoteReq) *RequestVoteResp {
	buildResp := func(grant bool) *RequestVoteResp {
		return &RequestVoteResp {
			VoteGranted: grant,
			Term: f.currentTerm,
		}
	}

	if f.votedFor != InvalidId && f.votedFor != req.CandidateId {
		return buildResp(false)
	}

	if req.Term < f.currentTerm {
		return buildResp(false)
	}

	if lastIdx := len(f.log) - 1; lastIdx >= 0 {
		lastEntry := f.log[lastIdx]
		if lastEntry.Term > req.LastLogTerm || (lastEntry.Term == req.LastLogTerm && lastEntry.Idx > req.LastLogIndex) {
			return buildResp(false)
		}
	}

	f.currentTerm = req.Term
	f.votedFor = req.CandidateId

	return buildResp(true)
}

type Candidate RaftBase

type Leader struct {
	base RaftBase
	nextIndex []Index
	matchIndex []Index
}

