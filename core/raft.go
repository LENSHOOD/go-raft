package core

import "math/rand"

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
	TakeAction(tor TickOrReq) (obj RaftObject, resp interface{})
}

type Cluster struct {
	Me Id
	Others []Id
}

type Config struct {
	cluster Cluster
	leader Id
	electionTimeoutMin int64
	electionTimeoutMax int64
	electionTimeout int64
	tickCnt int64
}

type RaftBase struct {
	cfg Config
	currentTerm Term
	votedFor Id
	commitIndex Index
	lastApplied Index
	log []Entry
}

func newRaftBase(cfg Config) RaftBase {
	return RaftBase {
		cfg: cfg,
		currentTerm: 0,
		votedFor: InvalidId,
		commitIndex: 0,
		lastApplied: 0,
		log: make([]Entry, 0),
	}
}

type Follower RaftBase

func NewFollower(cfg Config) *Follower {
	follower := Follower(newRaftBase(cfg))
	return &follower
}

func (f *Follower) TakeAction(tor TickOrReq) (obj RaftObject, resp interface{}) {
	if tor.Req == nil {
		f.cfg.tickCnt++

		if f.cfg.tickCnt == f.cfg.electionTimeout {
			f.cfg.tickCnt = 0
			f.cfg.electionTimeout = rand.Int63n(f.cfg.electionTimeoutMax - f.cfg.electionTimeoutMin) + f.cfg.electionTimeoutMin
			return f.toCandidate(), nil
		}

		return f, nil
	}

	f.cfg.tickCnt = 0
	switch tor.Req.(type) {
	case *RequestVoteReq:
		return f, f.vote(tor.Req.(*RequestVoteReq))
	case *AppendEntriesReq:
		return f, f.append(tor.Req.(*AppendEntriesReq))
	default:
		panic("Shouldn't goes here")
	}
}

// vote for some candidate, rules:
//     1. if term < currentTerm, not vote
//     2. first-come-first-served, if already vote to candidate-a,
//        then not vote to candidate-b, clear voteFor when term > currentTerm
//     3. if follower's last log entry's term or index bigger than candidate, not vote
func (f *Follower) vote(req *RequestVoteReq) *RequestVoteResp {
	buildResp := func(grant bool) *RequestVoteResp {
		return &RequestVoteResp {
			VoteGranted: grant,
			Term: f.currentTerm,
		}
	}

	if req.Term < f.currentTerm {
		return buildResp(false)
	}

	if req.Term == f.currentTerm && f.votedFor != InvalidId && f.votedFor != req.CandidateId {
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

// append log from leader, rules:
//     1. if term < currentTerm, not append
//     2. if follower's log doesn't contain an entry at prevLogIndex whose term matches prevLogTerm, not append
//     3. if an existing entry conflicts with a new one (same index but different terms),
//        delete the existing entry and all that follow it
//     4. append any new entries not already in the log
//     5. if leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
func (f * Follower) append(req *AppendEntriesReq) *AppendEntriesResp {
	buildResp := func(success bool) *AppendEntriesResp {
		return &AppendEntriesResp {
			Term: f.currentTerm,
			Success: success,
		}
	}

	if req.Term < f.currentTerm {
		return buildResp(false)
	}
	f.currentTerm = req.Term

	matched, logPos := matchPrev(f.log, req.PrevLogTerm, req.PrevLogIndex)
	if !matched {
		return buildResp(false)
	}

	replicateBeginPos := 0
	for _, v := range req.Entries {
		if logPos == -1 || f.log[logPos] != v {
			break
		}

		logPos++
		replicateBeginPos++
	}

	f.log = append(f.log[:logPos+1], req.Entries[replicateBeginPos:]...)
	lastEntryIndex := f.log[len(f.log) - 1].Idx
	if lastEntryIndex < req.LeaderCommit {
		f.commitIndex = lastEntryIndex
	} else {
		f.commitIndex = req.LeaderCommit
	}

	// TODO: Apply cmd to state machine, consider add a apply channel. Apply from lastApplied to commitIndex
	f.lastApplied = f.commitIndex

	return buildResp(true)
}

func matchPrev(log []Entry, term Term, idx Index) (matched bool, logPos int) {
	if len(log) == 0 {
		return true, -1
	}

	for i := len(log) - 1; i >= 0; i-- {
		entry := log[i]
		if entry.Term < term {
			return false, -1
		}

		if entry.Term == term && entry.Idx == idx{
			return true, i
		}
	}

	return false, -1
}

func (f *Follower) toCandidate() *Candidate {
	return &Candidate {
		cfg: f.cfg,
		currentTerm: f.currentTerm + 1,
		votedFor: 0,
		commitIndex: f.commitIndex,
		lastApplied: f.lastApplied,
		log: f.log,
	}
}

type Candidate RaftBase

func (c *Candidate) TakeAction(tor TickOrReq) (obj RaftObject, resp interface{}) {
	return nil, nil
}

type Leader struct {
	base RaftBase
	nextIndex []Index
	matchIndex []Index
}

