package core

var heartbeatInterval = 3

type Leader struct {
	RaftBase
	heartbeatIntervalCnt int
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
	}
	return NullMsg
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

	return l
}
