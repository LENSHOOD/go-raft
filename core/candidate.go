package core

import "math/rand"

type Candidate struct{
	RaftBase
	voted map[Id]bool
}

func (c *Candidate) TakeAction(msg Msg) Msg {
	switch msg.tp {
	case Tick:
		c.cfg.tickCnt++

		if c.cfg.tickCnt == c.cfg.electionTimeout {
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

	case Resp:
		c.cfg.tickCnt = 0
		switch msg.payload.(type) {
		case *RequestVoteResp:
			resp := msg.payload.(*RequestVoteResp)
			if resp.Term < c.currentTerm {
				break
			}

			if resp.VoteGranted {
				c.voted[msg.from] = true
			} else if resp.Term > c.currentTerm {
				c.currentTerm = resp.Term
				return c.moveState(c.toFollower())
			}
		}

	case Req:
		c.cfg.tickCnt = 0
		switch msg.payload.(type) {
		case *AppendEntriesReq:
			req := msg.payload.(*AppendEntriesReq)
			if req.Term < c.currentTerm {
				break
			}

			if req.Term >= c.currentTerm {
				c.currentTerm = req.Term
				return c.moveState(c.toFollower())
			}

		case *RequestVoteReq:
			req := msg.payload.(*RequestVoteReq)
			if req.Term < c.currentTerm {
				break
			} else if req.Term > c.currentTerm {
				c.currentTerm = req.Term
				return c.moveState(c.toFollower())
			}
		}
	default:
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
		make(map[Id]bool),
	}

	electionTimeout := rand.Int63n(f.cfg.electionTimeoutMax-f.cfg.electionTimeoutMin) + f.cfg.electionTimeoutMin
	c.cfg.electionTimeout = electionTimeout
	// force candidate start election at first tick
	c.cfg.tickCnt = electionTimeout - 1

	// vote self
	c.votedFor = c.cfg.cluster.Me
	c.voted[c.cfg.cluster.Me] = true

	return c
}
