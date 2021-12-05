package core

import (
	"math/rand"
)

type Candidate struct {
	RaftBase
	voted map[Id]bool
}

func (c *Candidate) TakeAction(msg Msg) Msg {
	switch msg.Tp {
	case Tick:
		c.cfg.tickCnt++

		if c.cfg.tickCnt == c.cfg.electionTimeout {
			c.currentTerm++

			// reset timeout
			c.cfg.electionTimeout = rand.Int63n(c.cfg.electionTimeoutMax-c.cfg.electionTimeoutMin) + c.cfg.electionTimeoutMin
			c.cfg.tickCnt = 0

			// send vote req
			lastEntry := c.getLastEntry()
			return c.broadcastReq(
				&RequestVoteReq{
					Term:         c.currentTerm,
					CandidateId:  c.cfg.cluster.Me,
					LastLogIndex: lastEntry.Idx,
					LastLogTerm:  lastEntry.Term,
				})
		}

	case Rpc:
		c.cfg.tickCnt = 0

		recvTerm := msg.Payload.(TermHolder).GetTerm()
		if recvTerm < c.currentTerm {
			return NullMsg
		} else if recvTerm > c.currentTerm {
			c.currentTerm = recvTerm
			return c.moveState(c.toFollower())
		}

		switch msg.Payload.(type) {
		case *AppendEntriesReq:
			return c.moveState(c.toFollower())
		case *RequestVoteResp:
			resp := msg.Payload.(*RequestVoteResp)
			c.voted[msg.From] = resp.VoteGranted

			voteCnt := 0
			for _, v := range c.voted {
				if v {
					voteCnt++
				}
			}

			if c.cfg.cluster.meetMajority(voteCnt) {
				return c.moveState(c.toLeader())
			}
		}
	}

	// return null for meaningless msg
	return NullMsg
}

func (c *Candidate) toFollower() *Follower {
	f := NewFollower(c.cfg, c.sm)
	f.currentTerm = c.currentTerm
	f.log = c.log
	f.commitIndex = c.commitIndex
	f.lastApplied = c.lastApplied

	return f
}

func (c *Candidate) toLeader() *Leader {
	return NewLeader(c)
}

func NewCandidate(f *Follower) *Candidate {
	c := &Candidate{
		RaftBase{
			cfg:         f.cfg,
			currentTerm: f.currentTerm,
			votedFor:    InvalidId,
			commitIndex: f.commitIndex,
			lastApplied: f.lastApplied,
			log:         f.log,
			sm:          f.sm,
		},
		make(map[Id]bool),
	}

	// force candidate start election at first tick
	c.cfg.tickCnt = c.cfg.electionTimeout - 1

	// vote self
	c.votedFor = c.cfg.cluster.Me

	return c
}
