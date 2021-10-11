package core

var heartbeatInterval = 3

type clientCtx struct {
	clientId Id
}

type Leader struct {
	RaftBase
	heartbeatIntervalCnt int
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
	if l.heartbeatIntervalCnt < heartbeatInterval {
		return NullMsg
	}

	l.heartbeatIntervalCnt = 0

	// send heartbeat (empty append log rpc)
	return l.broadcastReq(&AppendEntriesReq{
		Term:         l.currentTerm,
		LeaderId:     l.cfg.cluster.Me,
		PrevLogIndex: InvalidIndex,
		PrevLogTerm:  InvalidTerm,
		Entries:      []Entry{},
		LeaderCommit: l.commitIndex,
	})
}

func (l *Leader) appendLogFromCmd(from Id, cmd Command) Msg {
	lastEntry := l.getLastEntry()

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

	l.matchIndex[msg.From]++
	currFollowerMatchedIdx := l.matchIndex[msg.From]

	majorityCnt := 1
	for _, v := range l.matchIndex {
		if v >= currFollowerMatchedIdx {
			majorityCnt++
		}
	}

	if majorityCnt >= l.cfg.cluster.majorityCnt() {
		// send resp only if there is a not-yet-response cmd req existed
		if v, ok := l.clientCtxs[currFollowerMatchedIdx]; ok {
			l.commitIndex = currFollowerMatchedIdx
			res := l.applyCmdToStateMachine()

			delete(l.clientCtxs, currFollowerMatchedIdx)
			return l.pointReq(v.clientId, &CmdResp{Result: res, Success: true})
		}
	}

	return NullMsg
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
