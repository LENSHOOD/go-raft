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
	switch msg.tp {
	case Tick:
		l.heartbeatIntervalCnt++
		if l.heartbeatIntervalCnt >= heartbeatInterval {
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
	case Cmd:
		switch msg.payload.(type) {
		case *CmdReq:
			return l.broadcastReq(l.appendLogFromCmd(msg.from, msg.payload.(*CmdReq).Cmd))
		}
	case Rpc:
		recvTerm := msg.payload.(TermHolder).GetTerm()
		if recvTerm < l.currentTerm {
			return NullMsg
		} else if recvTerm > l.currentTerm {
			l.currentTerm = recvTerm
			return l.moveState(l.toFollower(msg.from))
		}

		switch msg.payload.(type) {
		case *AppendEntriesResp:
			resp := msg.payload.(*AppendEntriesResp)
			if resp.Success {
				l.matchIndex[msg.from]++
			}

			majorityCnt := 1
			self := l.matchIndex[msg.from]
			for _, v := range l.matchIndex {
				if v >= self {
					majorityCnt++
				}
			}

			if majorityCnt >= l.cfg.cluster.majorityCnt() {
				if v, ok := l.clientCtxs[self]; ok {
					l.commitIndex = self
					// TODO: Apply cmd to state machine, consider add a apply channel. Apply from lastApplied to commitIndex
					l.lastApplied = l.commitIndex

					delete(l.clientCtxs, self)
					return l.pointReq(v.clientId, &CmdResp{Success: true})
				}
			}
		}
	}
	return NullMsg
}

func (l *Leader) appendLogFromCmd(from Id, cmd Command) *AppendEntriesReq {
	lastIndex := InvalidIndex
	lastTerm := InvalidTerm
	if len(l.log) >= 0 {
		lastIndex = l.log[len(l.log)-1].Idx
		lastTerm = l.log[len(l.log)-1].Term
	}

	newEntry := Entry{
		Term: l.currentTerm,
		Idx:  lastIndex + 1,
		Cmd:  cmd,
	}
	l.log = append(l.log, newEntry)
	l.clientCtxs[newEntry.Idx] = clientCtx{clientId: from}

	return &AppendEntriesReq{
		Term:         l.currentTerm,
		LeaderId:     l.cfg.leader,
		PrevLogIndex: lastIndex,
		PrevLogTerm:  lastTerm,
		Entries:      []Entry{newEntry},
		LeaderCommit: l.commitIndex,
	}
}

func (l *Leader) toFollower(newLeader Id) *Follower {
	f := NewFollower(l.cfg)
	f.currentTerm = l.currentTerm
	f.log = l.log
	f.commitIndex = l.commitIndex
	f.lastApplied = l.lastApplied
	f.votedFor = InvalidId
	f.cfg.leader = newLeader

	return f
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
		},
		0,
		make(map[Index]clientCtx),
		make(map[Id]Index),
		make(map[Id]Index),
	}

	lastLogIndex := InvalidIndex
	if len(l.log) > 0 {
		lastLogIndex = l.log[len(l.log)-1].Idx
	}
	for v := range l.cfg.cluster.Others {
		l.nextIndex[Id(v)] = lastLogIndex
		l.matchIndex[Id(v)] = InvalidIndex
	}

	l.cfg.leader = l.cfg.cluster.Me
	return l
}
