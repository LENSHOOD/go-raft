package core

import "math/rand"

type Candidate struct{ RaftBase }

func (c *Candidate) TakeAction(msg Msg) Msg {
	switch msg.tp {
	case Tick:
		c.cfg.tickCnt++

		if c.cfg.tickCnt == c.cfg.electionTimeout {
			// vote myself
			c.votedFor = c.cfg.cluster.Me

			// send vote req
			lastLogIndex := InvalidIndex
			lastLogTerm := InvalidTerm
			if lastIdx := len(c.log) - 1; lastIdx >= 0 {
				lastLogIndex = c.log[lastIdx].Idx
				lastLogTerm = c.log[lastIdx].Term
			}
			return c.broadcastReq(
				&RequestVoteReq{
					Term:         c.currentTerm,
					CandidateId:  c.cfg.cluster.Me,
					LastLogIndex: lastLogIndex,
					LastLogTerm:  lastLogTerm,
				})
		}

	default:
	}

	// return null for meaningless msg
	return NullMsg
}

func NewCandidate(f *Follower) *Candidate {
	c := &Candidate{
		RaftBase{
			cfg:         f.cfg,
			currentTerm: f.currentTerm + 1,
			votedFor:    InvalidId,
			commitIndex: f.commitIndex,
			lastApplied: f.lastApplied,
			log:         f.log,
		},
	}

	electionTimeout := rand.Int63n(f.cfg.electionTimeoutMax-f.cfg.electionTimeoutMin) + f.cfg.electionTimeoutMin
	c.cfg.electionTimeout = electionTimeout
	// force candidate start election at first tick
	c.cfg.tickCnt = electionTimeout - 1

	return c
}
