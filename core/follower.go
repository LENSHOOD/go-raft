package core

type Follower struct{ RaftBase }

func NewFollower(cfg Config, sm StateMachine) *Follower {
	follower := Follower{newRaftBase(cfg, sm)}
	return &follower
}

func (f *Follower) TakeAction(msg Msg) Msg {
	switch msg.Tp {
	case Tick:
		f.cfg.tickCnt++

		if f.cfg.tickCnt == f.cfg.electionTimeout {
			return f.moveState(f.toCandidate())
		}

	case Rpc:
		f.cfg.tickCnt = 0

		recvTerm := msg.Payload.(TermHolder).GetTerm()
		// update term and clear vote when receive newer term
		if recvTerm > f.currentTerm {
			f.currentTerm = recvTerm
			f.votedFor = InvalidId
		}

		switch msg.Payload.(type) {
		case *RequestVoteReq:
			return f.Resp(msg.From, f.vote(msg.Payload.(*RequestVoteReq)))
		case *AppendEntriesReq:
			return f.Resp(msg.From, f.append(msg.Payload.(*AppendEntriesReq)))
		}
	case Cmd:
		return f.Resp(msg.From,
			&CmdResp{
				Result:  f.cfg.leader,
				Success: false,
			})
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

	if f.votedFor != InvalidId && f.votedFor != req.CandidateId {
		return buildResp(false)
	}

	if lastIdx := len(f.log) - 1; lastIdx >= 0 {
		lastEntry := f.log[lastIdx]
		if lastEntry.Term > req.LastLogTerm || (lastEntry.Term == req.LastLogTerm && lastEntry.Idx > req.LastLogIndex) {
			return buildResp(false)
		}
	}

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
		if f.commitIndex < req.LeaderCommit {
			f.commitIndex = req.LeaderCommit
		}

		return buildResp(false)
	}

	matched, logPos := matchPrev(f.log, req.PrevLogTerm, req.PrevLogIndex)
	if !matched {
		return buildResp(false)
	}

	// no need to append log if receive heartbeat
	if len(req.Entries) != 0 {
		replicateBeginPos := 0
		for _, v := range req.Entries {
			if logPos == -1 || f.log[logPos] != v {
				break
			}

			logPos++
			replicateBeginPos++
		}

		f.log = append(f.log[:logPos+1], req.Entries[replicateBeginPos:]...)
	}

	f.tryApplyCmd(req.LeaderCommit)
	return buildResp(true)
}

func matchPrev(log []Entry, term Term, idx Index) (matched bool, logPos int) {
	// first entry
	if term == InvalidTerm && idx == InvalidIndex {
		return true, -1
	}

	for i := len(log) - 1; i >= 0; i-- {
		entry := log[i]
		if entry.Term < term {
			return false, -2
		}

		if entry.Term == term && entry.Idx == idx {
			return true, i
		}
	}

	return false, -2
}

func (f *Follower) tryApplyCmd(leaderCommitIdx Index) {
	lastEntry := f.getLastEntry()
	if lastEntry == InvalidEntry {
		return
	}

	// do config change no matter this entry has been committed or not
	if configChangedCmd, ok := lastEntry.Cmd.(*ConfigChangeCmd); ok {
		f.cfg.cluster.replaceTo(configChangedCmd.Members)
	}

	prevCommitted := f.commitIndex
	if lastEntry.Idx < leaderCommitIdx {
		f.commitIndex = lastEntry.Idx
	} else {
		f.commitIndex = leaderCommitIdx
	}

	if f.commitIndex == InvalidIndex || f.lastApplied == f.commitIndex {
		return
	}

	for i := prevCommitted + 1; i <= f.commitIndex; i++ {
		f.applyCmdToStateMachine()
	}
}

func (f *Follower) toCandidate() *Candidate {
	return NewCandidate(f)
}
