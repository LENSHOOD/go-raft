package core

var heartbeatDivideFactor int64 = 2

type clientCtx struct {
	clientId Address
}

type Leader struct {
	RaftBase
	heartbeatIntervalCnt int64
	clientCtxs           map[Index]clientCtx
	nextIndex            map[Address]Index
	matchIndex           map[Address]Index
	inTransfer           bool
}

func (l *Leader) TakeAction(msg Msg) Msg {
	switch msg.Tp {
	case Tick:
		if l.inTransfer {
			l.cfg.tickCnt++

			// first tick after in transfer be set
			if l.cfg.tickCnt == 1 {
				if timeoutNowReq, ok := l.tryTransferLeadership(); ok {
					return timeoutNowReq
				}

				// reset tickCnt to let it retry transfer leader at next tick
				l.cfg.tickCnt = 0
			}

			l.tryClearInTransfer()
		}

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
			// ignore vote request except for leader transfer request, to prevent disruptive server
			if voteReq, ok := msg.Payload.(*RequestVoteReq); ok && !voteReq.LeaderTransfer {
				return NullMsg
			}
			l.currentTerm = recvTerm
			return l.moveState(l.toFollower())
		}

		switch msg.Payload.(type) {
		case *AppendEntriesResp:
			return l.dealWithAppendLogResp(msg)
		}
	}

	return NullMsg
}

func (l *Leader) tryClearInTransfer() {
	if l.cfg.tickCnt == l.cfg.electionTimeout {
		l.inTransfer = false
		l.cfg.tickCnt = 0
	}
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

func (l *Leader) appendLogFromCmd(from Address, cmd Command) Msg {
	if l.inTransfer {
		// TODO: format result, should not return nil
		return l.pointReq(from, &CmdResp{Result: nil, Success: false})
	}

	lastEntry := l.getLastEntry()

	// do config change no matter this entry has been committed or not
	if configChangedCmd, ok := cmd.(*ConfigChangeCmd); ok {
		// once any uncommitted config change entry found, then abort current process
		for i := l.commitIndex + 1; i <= lastEntry.Idx; i++ {
			if _, exist := l.getEntryByIdx(i).Cmd.(*ConfigChangeCmd); exist {
				// TODO: format result, should not return nil
				return l.pointReq(from, &CmdResp{Result: nil, Success: false})
			}
		}

		// save previous member in case of roll back
		configChangedCmd.PrevMembers = make([]Address, len(l.cfg.cluster.Members))
		_ = copy(configChangedCmd.PrevMembers, l.cfg.cluster.Members)

		l.cfg.cluster.replaceTo(configChangedCmd.Members)

		l.maintainNextAndMatchIdxForChangedServer(configChangedCmd, lastEntry.Idx)
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

func (l *Leader) maintainNextAndMatchIdxForChangedServer(configChangedCmd *ConfigChangeCmd, lastIdx Index) {
	// when add new server, init nextIndex and matchIndex to that server
	if len(configChangedCmd.Members) > len(configChangedCmd.PrevMembers) {
		hashSet := make(map[Address]struct{})
		for _, member := range configChangedCmd.PrevMembers {
			hashSet[member] = struct{}{}
		}
		for _, member := range configChangedCmd.Members {
			if _, exist := hashSet[member]; !exist {
				// lastEntry.Idx + 2 count this config change entry in
				l.nextIndex[member] = lastIdx + 2
				l.matchIndex[member] = InvalidIndex
				break
			}
		}
	}

	// TODO: found removed server, delete related nextIndex and matchIndex
}

func (l *Leader) toFollower() *Follower {
	f := NewFollower(l.cfg, l.sm)
	f.currentTerm = l.currentTerm
	f.log = l.log
	f.commitIndex = l.commitIndex
	f.lastApplied = l.lastApplied
	f.votedFor = InvalidId
	f.cfg.leader = InvalidId

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

	majorityCnt := 0
	for _, v := range l.matchIndex {
		if v >= currFollowerMatchedIdx {
			majorityCnt++
		}
	}

	// only commit & apply entry that belong to current term
	// (if received previous entry that replicated to majority, then that entry will be committed
	// right after first current term entry committed)
	currFollowerMatchedTerm := l.getEntryByIdx(currFollowerMatchedIdx).Term
	var idSet []Address
	payloadMap := make(map[Address]interface{})
	if l.cfg.cluster.meetMajority(majorityCnt) && currFollowerMatchedTerm == l.currentTerm {
		// send resp only if there is a not-yet-respond cmd req existed
		for i := l.commitIndex + 1; i <= currFollowerMatchedIdx; i++ {
			// when config change lead to self eviction
			if _, ok := l.getEntryByIdx(i).Cmd.(*ConfigChangeCmd); ok && !l.isMeIncludedInCluster() {
				l.inTransfer = true
			}

			l.commitIndex = i
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

	return l.composedReq(idSet, func(to Address) interface{} {
		return &CmdResp{Result: payloadMap[to], Success: true}
	})
}

func (l *Leader) resendAppendLogWithDecreasedIdx(followerId Address) Msg {
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

func (l *Leader) tryTransferLeadership() (msg Msg, eligibleToTransfer bool) {
	idxShouldMatch := l.getLastEntry().Idx
	for id, index := range l.matchIndex {
		if index == idxShouldMatch {
			return l.pointReq(id, &TimeoutNowReq{Term: l.currentTerm}), true
		}
	}

	return NullMsg, false
}

func (l *Leader) isMeIncludedInCluster() bool {
	for _, member := range l.cfg.cluster.Members {
		if l.cfg.cluster.Me == member {
			return true
		}
	}

	return false
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
		make(map[Address]Index),
		make(map[Address]Index),
		false,
	}

	lastEntry := l.getLastEntry()
	for _, v := range l.cfg.cluster.Members {
		if v == l.cfg.cluster.Me {
			continue
		}

		// next idx should be the last valid id +1
		l.nextIndex[v] = lastEntry.Idx + 1
		l.matchIndex[v] = InvalidIndex
	}

	l.cfg.leader = l.cfg.cluster.Me
	l.cfg.tickCnt = 0
	return l
}
