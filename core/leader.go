package core

type Leader struct {
	RaftBase
	nextIndex  map[Id]Index
	matchIndex map[Id]Index
}

func (l *Leader) TakeAction(msg Msg) Msg {
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
		make(map[Id]Index),
		make(map[Id]Index),
	}

	lastLogIndex := InvalidIndex
	if len(l.log) > 0 {
		lastLogIndex = l.log[len(l.log) - 1].Idx
	}
	for v := range l.cfg.cluster.Others {
		l.nextIndex[Id(v)] = lastLogIndex
		l.matchIndex[Id(v)] = InvalidIndex
	}

	return l
}