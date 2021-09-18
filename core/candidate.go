package core

import "math/rand"

type Candidate struct{ RaftBase }

func (c *Candidate) TakeAction(msg Msg) Msg {
	return Msg{}
}

func NewCandidate(f *Follower) *Candidate {
	c := &Candidate{
		RaftBase{
			cfg:         f.cfg,
			currentTerm: f.currentTerm + 1,
			votedFor:    0,
			commitIndex: f.commitIndex,
			lastApplied: f.lastApplied,
			log:         f.log,
		},
	}

	electionTimeout := rand.Int63n(f.cfg.electionTimeoutMax-f.cfg.electionTimeoutMin) + f.cfg.electionTimeoutMin
	c.cfg.electionTimeout = electionTimeout
	// force candidate start election at first tick
	c.cfg.tickCnt = electionTimeout

	return c
}
