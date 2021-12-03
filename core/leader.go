package core

var heartbeatDivideFactor int64 = 2

type clientCtx struct {
	clientId Id
}

type Leader struct {
	RaftBase
	heartbeatIntervalCnt int64
	clientCtxs           map[Index]clientCtx
	nextIndex            map[Id]Index
	matchIndex           map[Id]Index
}

func (l *Leader) TakeAction(msg Msg) Msg {
	switch msg.Tp {
	case Tick:
		return l.sendHeartbeat()

	case Cmd:
		switch msg.Payload.(type) {
		case *CmdReq:
			return l.appendLogFromCmd(msg.From, msg.Payload.(*CmdReq).Cmd)
		}

	case Rpc:
		recvTerm := msg.Payload.(TermHolder).GetTerm()
		if recvTerm < l.currentTerm {
			return NullMsg
		} else if recvTerm > l.currentTerm {
			l.currentTerm = recvTerm
			return l.moveState(l.toFollower(msg.From))
		}

		switch msg.Payload.(type) {
		case *AppendEntriesResp:
			return l.dealWithAppendLogResp(msg)
		}
	}

	return NullMsg
}

func (l *Leader) sendHeartbeat() Msg {
	l.heartbeatIntervalCnt++
	if l.heartbeatIntervalCnt < (l.cfg.electionTimeoutMin / heartbeatDivideFactor) {
		return NullMsg
	}

	l.heartbeatIntervalCnt = 0

	// send heartbeat (empty append log rpc)
	lastEntry := l.getLastEntry()
	return l.broadcastReq(&AppendEntriesReq{
		Term:         l.currentTerm,
		LeaderId:     l.cfg.cluster.Me,
		PrevLogIndex: lastEntry.Idx,
		PrevLogTerm:  lastEntry.Term,
		Entries:      []Entry{},
		LeaderCommit: l.commitIndex,
	})
}

func (l *Leader) appendLogFromCmd(from Id, cmd Command) Msg {
	lastEntry := l.getLastEntry()

	// do config change no matter this entry has been committed or not
	if configChangedCmd, ok := cmd.(*ConfigChangeCmd); ok {
		// once any uncommitted config change entry found, then abort current process
		for i := l.commitIndex + 1; i <= lastEntry.Idx; i++ {
			if _, exist := l.getEntryByIdx(i).Cmd.(*ConfigChangeCmd); exist {
				return l.pointReq(from, &CmdResp{Result: nil, Success: false})
			}
		}

		// save previous member in case of roll back
		prevMember := make([]Id, len(l.cfg.cluster.Others)+1)
		_ = copy(prevMember, append(l.cfg.cluster.Others, l.cfg.cluster.Me))
		configChangedCmd.PrevMembers = prevMember

		l.cfg.cluster.replaceTo(configChangedCmd.Members)
	}

	newEntry := Entry{
		Term: l.currentTerm,
		Idx:  lastEntry.Idx + 1,
		Cmd:  cmd,
	}
	l.log = append(l.log, newEntry)
	l.clientCtxs[newEntry.Idx] = clientCtx{clientId: from}

	return l.broadcastReq(&AppendEntriesReq{
		Term:         l.currentTerm,
		LeaderId:     l.cfg.leader,
		PrevLogIndex: lastEntry.Idx,
		PrevLogTerm:  lastEntry.Term,
		Entries:      []Entry{newEntry},
		LeaderCommit: l.commitIndex,
	})
}

func (l *Leader) toFollower(newLeader Id) *Follower {
	f := NewFollower(l.cfg, l.sm)
	f.currentTerm = l.currentTerm
	f.log = l.log
	f.commitIndex = l.commitIndex
	f.lastApplied = l.lastApplied
	f.votedFor = InvalidId
	f.cfg.leader = newLeader

	return f
}

func (l *Leader) dealWithAppendLogResp(msg Msg) Msg {
	resp := msg.Payload.(*AppendEntriesResp)
	if !resp.Success {
		return l.resendAppendLogWithDecreasedIdx(msg.From)
	}

	// matchedId++, nextId++, up to last idx
	if l.matchIndex[msg.From] < l.getLastEntry().Idx {
		l.matchIndex[msg.From]++
	}
	l.nextIndex[msg.From] = l.matchIndex[msg.From] + 1
	currFollowerMatchedIdx := l.matchIndex[msg.From]

	majorityCnt := 1
	for _, v := range l.matchIndex {
		if v >= currFollowerMatchedIdx {
			majorityCnt++
		}
	}

	// only commit & apply entry that belong to current term
	// (if received previous entry that replicated to majority, then that entry will be committed
	// right after first current term entry committed)
	currFollowerMatchedTerm := l.getEntryByIdx(currFollowerMatchedIdx).Term
	var idSet []Id
	payloadMap := make(map[Id]interface{})
	if majorityCnt >= l.cfg.cluster.majorityCnt() && currFollowerMatchedTerm == l.currentTerm {
		// send resp only if there is a not-yet-response cmd req existed
		for i := l.commitIndex + 1; i <= currFollowerMatchedIdx; i++ {
			l.commitIndex = i
			// TODO: do not return to client when config change merge phase committed
			res := l.applyCmdToStateMachine()

			v, exist := l.clientCtxs[i]
			if !exist {
				continue
			}

			delete(l.clientCtxs, i)
			idSet = append(idSet, v.clientId)
			payloadMap[v.clientId] = res
		}
	}

	if len(idSet) == 0 {
		return NullMsg
	}

	return l.composedReq(idSet, func(to Id) interface{} {
		return &CmdResp{Result: payloadMap[to], Success: true}
	})
}

func (l *Leader) resendAppendLogWithDecreasedIdx(followerId Id) Msg {
	stepBackedNextIdx := l.nextIndex[followerId] - 1
	l.nextIndex[followerId] = stepBackedNextIdx

	prevEntryIdx := -1
	for i := len(l.log) - 1; i >= 0; i-- {
		if l.log[i].Idx == stepBackedNextIdx-2 {
			prevEntryIdx = i
		}
	}

	prevIdx := InvalidIndex
	prevTerm := InvalidTerm
	if prevEntryIdx != -1 {
		prevIdx = l.log[prevEntryIdx].Idx
		prevTerm = l.log[prevEntryIdx].Term
	}

	return l.pointReq(followerId, &AppendEntriesReq{
		Term:         l.currentTerm,
		LeaderId:     l.cfg.leader,
		PrevLogIndex: prevIdx,
		PrevLogTerm:  prevTerm,
		Entries:      l.log[prevEntryIdx+1:],
		LeaderCommit: l.commitIndex,
	})
}

func NewLeader(c *Candidate) *Leader {
	l := &Leader{
		RaftBase{
			cfg:         c.cfg,
			currentTerm: c.currentTerm,
			votedFor:    InvalidId,
			commitIndex: c.commitIndex,
			lastApplied: c.lastApplied,
			log:         c.log,
			sm:          c.sm,
		},
		0,
		make(map[Index]clientCtx),
		make(map[Id]Index),
		make(map[Id]Index),
	}

	lastEntry := l.getLastEntry()
	for _, v := range l.cfg.cluster.Others {
		// next idx should be the last valid id +1
		l.nextIndex[v] = lastEntry.Idx + 1
		l.matchIndex[v] = InvalidIndex
	}

	l.cfg.leader = l.cfg.cluster.Me
	return l
}
