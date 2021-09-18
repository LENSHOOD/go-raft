package core

import "math/rand"

type Follower struct{ RaftBase }

func NewFollower(cfg Config) *Follower {
	follower := Follower{newRaftBase(cfg)}
	return &follower
}

func (f *Follower) TakeAction(msg Msg) Msg {
	switch msg.tp {
	case Tick:
		f.cfg.tickCnt++

		if f.cfg.tickCnt == f.cfg.electionTimeout {
			f.cfg.tickCnt = 0
			f.cfg.electionTimeout = rand.Int63n(f.cfg.electionTimeoutMax-f.cfg.electionTimeoutMin) + f.cfg.electionTimeoutMin
			return f.moveState(f.toCandidate())
		}

	case Req:
		f.cfg.tickCnt = 0
		switch msg.payload.(type) {
		case *RequestVoteReq:
			return f.Resp(msg.from, f.vote(msg.payload.(*RequestVoteReq)))
		case *AppendEntriesReq:
			return f.Resp(msg.from, f.append(msg.payload.(*AppendEntriesReq)))
		}

	default:
	}

	// return null for meaningless msg
	return NullMsg
}

// vote for some candidate, rules:
//     1. if term < currentTerm, not vote
//     2. first-come-first-served, if already vote to candidate-a,
//        then not vote to candidate-b, clear voteFor when term > currentTerm
//     3. if follower's last log entry's term or index bigger than candidate, not vote
func (f *Follower) vote(req *RequestVoteReq) *RequestVoteResp {
	buildResp := func(grant bool) *RequestVoteResp {
		return &RequestVoteResp{
			VoteGranted: grant,
			Term:        f.currentTerm,
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
func (f *Follower) append(req *AppendEntriesReq) *AppendEntriesResp {
	buildResp := func(success bool) *AppendEntriesResp {
		return &AppendEntriesResp{
			Term:    f.currentTerm,
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
	lastEntryIndex := f.log[len(f.log)-1].Idx
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

		if entry.Term == term && entry.Idx == idx {
			return true, i
		}
	}

	return false, -1
}

func (f *Follower) toCandidate() *Candidate {
	return &Candidate{
		RaftBase{
			cfg:         f.cfg,
			currentTerm: f.currentTerm + 1,
			votedFor:    0,
			commitIndex: f.commitIndex,
			lastApplied: f.lastApplied,
			log:         f.log,
		},
	}
}