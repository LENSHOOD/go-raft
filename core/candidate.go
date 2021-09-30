package core

import "math/rand"

type Candidate struct {
	RaftBase
	voted map[Id]bool
}

func (c *Candidate) TakeAction(msg Msg) Msg {
	switch msg.tp {
	case Tick:
		c.cfg.tickCnt++

		if c.cfg.tickCnt == c.cfg.electionTimeout {
			c.currentTerm++

			// reset timeout
			c.cfg.electionTimeout = rand.Int63n(c.cfg.electionTimeoutMax-c.cfg.electionTimeoutMin) + c.cfg.electionTimeoutMin

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

	case Rpc:
		c.cfg.tickCnt = 0

		recvTerm := msg.payload.(TermHolder).GetTerm()
		if recvTerm < c.currentTerm {
			return NullMsg
		} else if recvTerm > c.currentTerm {
			c.currentTerm = recvTerm
			return c.moveState(c.toFollower())
		}

		switch msg.payload.(type) {
		case *RequestVoteResp:
			resp := msg.payload.(*RequestVoteResp)
			c.voted[msg.from] = resp.VoteGranted

			voteCnt := 0
			for v := range c.cfg.cluster.Others {
				if c.voted[Id(v)] {
					voteCnt++
				}
			}

			majorityCnt := (len(c.cfg.cluster.Others)+1)/2 + 1
			if voteCnt+1 >= majorityCnt {
				return c.moveState(c.toLeader())
			}
		}
	}

	// return null for meaningless msg
	return NullMsg
}

func (c *Candidate) toFollower() *Follower {
	f := NewFollower(c.cfg)
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
		},
		make(map[Id]bool),
	}

	// force candidate start election at first tick
	c.cfg.tickCnt = c.cfg.electionTimeout - 1

	// vote self
	c.votedFor = c.cfg.cluster.Me
	c.voted[c.cfg.cluster.Me] = true

	return c
}
