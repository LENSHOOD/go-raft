package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestCandidateCanStartElection(c *C) {
	// given
	cfg := Config {
		cluster: Cluster {
			Me:     1,
			Others: []Id{2, 3},
		},
		electionTimeoutMin: 3,
		electionTimeoutMax: 10,
		electionTimeout: 3,
		tickCnt: 0,
	}

	f := NewFollower(cfg)
	f.currentTerm = 4
	f.log = append(f.log, Entry{Term: 1, Idx: 0, Cmd: ""}, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	cand := f.toCandidate()
	tick := Msg { tp: Tick }

	// when
	res := cand.TakeAction(tick)

	// then
	c.Assert(res.tp, Equals, Req)
	c.Assert(res.from, Equals, cand.cfg.cluster.Me)
	c.Assert(res.to, Equals, All)
	if rv, ok := res.payload.(*RequestVoteReq); !ok {
		c.Fail()
	} else {
		c.Assert(rv.Term, Equals, f.currentTerm + 1)
		c.Assert(rv.CandidateId, Equals, cand.cfg.cluster.Me)
		c.Assert(rv.LastLogIndex, Equals, f.log[len(f.log) - 1].Idx)
		c.Assert(rv.LastLogTerm, Equals, f.log[len(f.log) - 1].Term)
	}

	// self vote
	c.Assert(cand.votedFor, Equals, cand.cfg.cluster.Me)
}
