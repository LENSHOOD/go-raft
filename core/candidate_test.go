package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestCandidateCanStartElection(c *C) {
	// given
	f := NewFollower(commCfg)
	f.currentTerm = 4
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	cand := f.toCandidate()
	tick := Msg{tp: Tick}

	// when
	res := cand.TakeAction(tick)

	// then
	c.Assert(res.tp, Equals, Rpc)
	c.Assert(res.from, Equals, cand.cfg.cluster.Me)
	c.Assert(res.to, Equals, All)
	if rv, ok := res.payload.(*RequestVoteReq); !ok {
		c.Fail()
	} else {
		c.Assert(rv.Term, Equals, f.currentTerm+1)
		c.Assert(rv.CandidateId, Equals, cand.cfg.cluster.Me)
		c.Assert(rv.LastLogIndex, Equals, f.log[len(f.log)-1].Idx)
		c.Assert(rv.LastLogTerm, Equals, f.log[len(f.log)-1].Term)
	}

	// self vote
	c.Assert(cand.votedFor, Equals, cand.cfg.cluster.Me)
	c.Assert(cand.voted[cand.cfg.cluster.Me], Equals, true)
}

func (t *T) TestCandidateWillRecordVoteFromOtherResp(c *C) {
	// given
	voteFollowerId0 := commCfg.cluster.Others[1]
	voteFollowerId1 := commCfg.cluster.Others[3]

	cand := NewFollower(commCfg).toCandidate()
	cand.currentTerm = 1

	buildResp := func(id Id) Msg {
		return Msg{
			tp:   Rpc,
			from: id,
			to:   commCfg.cluster.Me,
			payload: &RequestVoteResp{
				Term:        1,
				VoteGranted: true,
			},
		}
	}

	// when
	_ = cand.TakeAction(buildResp(voteFollowerId0))
	_ = cand.TakeAction(buildResp(voteFollowerId1))

	// then
	c.Assert(cand.voted[voteFollowerId0], Equals, true)
	c.Assert(cand.voted[voteFollowerId1], Equals, true)
	c.Assert(cand.voted[cand.cfg.cluster.Me], Equals, true)

	c.Assert(cand.voted[commCfg.cluster.Others[0]], Equals, false)
	c.Assert(cand.voted[commCfg.cluster.Others[2]], Equals, false)
}

func (t *T) TestCandidateWillBackToFollowerWhenReceiveVoteRespNewTerm(c *C) {
	// given
	cand := NewFollower(commCfg).toCandidate()

	voteFollowerId0 := commCfg.cluster.Others[1]
	resp := Msg{
		tp:   Rpc,
		from: voteFollowerId0,
		to:   commCfg.cluster.Me,
		payload: &RequestVoteResp{
			Term:        cand.currentTerm + 1,
			VoteGranted: false,
		},
	}

	// when
	res := cand.TakeAction(resp)

	// then
	c.Assert(res.tp, Equals, MoveState)
	if f, ok := res.payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (resp.payload.(*RequestVoteResp)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestCandidateWillBackToFollowerWhenReceiveReqVoteWithNewTerm(c *C) {
	// given
	cand := NewFollower(commCfg).toCandidate()

	newCandidate := commCfg.cluster.Others[1]
	resp := Msg{
		tp:   Rpc,
		from: newCandidate,
		to:   commCfg.cluster.Me,
		payload: &RequestVoteReq{
			Term:        cand.currentTerm + 1,
			CandidateId: newCandidate,
		},
	}

	// when
	res := cand.TakeAction(resp)

	// then
	c.Assert(res.tp, Equals, MoveState)
	if f, ok := res.payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (resp.payload.(*RequestVoteReq)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestCandidateWillBackToFollowerWhenReceiveAppendReqNewTerm(c *C) {
	// given
	cand := NewFollower(commCfg).toCandidate()
	cand.currentTerm = 3

	req := Msg{
		tp: Rpc,
		payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  4,
			PrevLogIndex: 5,
			Entries:      []Entry{{Term: 4, Idx: 6, Cmd: ""}},
		},
	}

	// when
	res := cand.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, MoveState)
	if f, ok := res.payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (req.payload.(*AppendEntriesReq)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestCandidateShouldIgnoreAnyMsgThatTermOlderThanItself(c *C) {
	// given
	cand := NewFollower(commCfg).toCandidate()
	cand.currentTerm = 2


	// when
	msg1 := Msg{
		tp: Rpc,
		payload: &AppendEntriesReq{
			Term:         1,
		},
	}
	res1 := cand.TakeAction(msg1)

	msg2 := Msg{
		tp: Rpc,
		payload: &RequestVoteResp{
			Term:        1,
			VoteGranted: true,
		},
	}
	res2 := cand.TakeAction(msg2)

	msg3 := Msg{
		tp: Rpc,
		payload: &RequestVoteReq{
			Term:        1,
		},
	}
	res3 := cand.TakeAction(msg3)

	// then
	c.Assert(res1, Equals, NullMsg)
	c.Assert(res2, Equals, NullMsg)
	c.Assert(res3, Equals, NullMsg)
}

func (t *T) TestCandidateTriggerElectionTimeoutWithEmptyTick(c *C) {
	// given
	currTerm := Term(1)
	cand := NewFollower(commCfg).toCandidate()
	cand.currentTerm = currTerm
	cand.cfg.electionTimeout = 3
	cand.cfg.tickCnt = 2

	req := Msg{tp: Tick}

	// when
	res := cand.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Rpc)
	c.Assert(res.from, Equals, cand.cfg.cluster.Me)
	c.Assert(res.to, Equals, All)
	if rv, ok := res.payload.(*RequestVoteReq); !ok {
		c.Fail()
	} else {
		c.Assert(rv.Term, Equals, currTerm + 1)
		c.Assert(rv.CandidateId, Equals, cand.cfg.cluster.Me)
	}

	// re-random timeout
	legalElectionTimeout := cand.cfg.electionTimeout >= cand.cfg.electionTimeoutMin && cand.cfg.electionTimeout <= cand.cfg.electionTimeoutMax
	c.Assert(legalElectionTimeout, Equals, true)
}

func (t *T) TestCandidateForwardToLeaderWhenReceiveMajorityVotes(c *C) {
	// given
	voteFollowerId0 := commCfg.cluster.Others[1]
	voteFollowerId1 := commCfg.cluster.Others[3]

	cand := NewFollower(commCfg).toCandidate()
	cand.currentTerm = 3
	cand.log = append(cand.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 3, Idx: 5, Cmd: ""})

	buildResp := func(id Id) Msg {
		return Msg{
			tp:   Rpc,
			from: id,
			to:   commCfg.cluster.Me,
			payload: &RequestVoteResp{
				Term:        3,
				VoteGranted: true,
			},
		}
	}

	// when
	_ = cand.TakeAction(buildResp(voteFollowerId0))
	res := cand.TakeAction(buildResp(voteFollowerId1))

	// then
	c.Assert(len(cand.voted), Equals, 3)
	c.Assert(res.tp, Equals, MoveState)
	if l, ok := res.payload.(*Leader); ok {
		lastLogIndex := cand.log[len(cand.log) - 1].Idx
		for v := range cand.cfg.cluster.Others {
			c.Assert(l.nextIndex[Id(v)], Equals, lastLogIndex + 1)
			c.Assert(l.matchIndex[Id(v)], Equals, InvalidIndex)
		}
	} else {
		c.Fail()
		c.Logf("Should move to leader.")
	}
}