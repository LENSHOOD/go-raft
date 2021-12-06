package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestCandidateCanStartElection(c *C) {
	// given
	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	cand := f.toCandidate(false)
	tick := Msg{Tp: Tick}

	// when
	res := cand.TakeAction(tick)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.From, Equals, cand.cfg.cluster.Me)
	c.Assert(res.To, Equals, All)
	if rv, ok := res.Payload.(*RequestVoteReq); !ok {
		c.Fail()
	} else {
		c.Assert(rv.Term, Equals, f.currentTerm+1)
		c.Assert(rv.CandidateId, Equals, cand.cfg.cluster.Me)
		c.Assert(rv.LastLogIndex, Equals, f.log[len(f.log)-1].Idx)
		c.Assert(rv.LastLogTerm, Equals, f.log[len(f.log)-1].Term)
	}

	// self vote
	c.Assert(cand.votedFor, Equals, cand.cfg.cluster.Me)
}

func (t *T) TestCandidateWillRecordVoteFromOtherResp(c *C) {
	// given
	voteFollowerId0 := commCfg.cluster.Members[1]
	voteFollowerId1 := commCfg.cluster.Members[3]

	cand := NewFollower(commCfg, mockSm).toCandidate(false)
	cand.currentTerm = 1

	buildResp := func(id Id) Msg {
		return Msg{
			Tp:   Rpc,
			From: id,
			To:   commCfg.cluster.Me,
			Payload: &RequestVoteResp{
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

	c.Assert(cand.voted[commCfg.cluster.Members[2]], Equals, false)
	c.Assert(cand.voted[commCfg.cluster.Members[4]], Equals, false)
}

func (t *T) TestCandidateWillBackToFollowerWhenReceiveVoteRespNewTerm(c *C) {
	// given
	cand := NewFollower(commCfg, mockSm).toCandidate(false)

	voteFollowerId0 := commCfg.cluster.Members[1]
	resp := Msg{
		Tp:   Rpc,
		From: voteFollowerId0,
		To:   commCfg.cluster.Me,
		Payload: &RequestVoteResp{
			Term:        cand.currentTerm + 1,
			VoteGranted: false,
		},
	}

	// when
	res := cand.TakeAction(resp)

	// then
	c.Assert(res.Tp, Equals, MoveState)
	if f, ok := res.Payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (resp.Payload.(*RequestVoteResp)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestCandidateWillBackToFollowerWhenReceiveReqVoteWithNewTerm(c *C) {
	// given
	cand := NewFollower(commCfg, mockSm).toCandidate(false)

	newCandidate := commCfg.cluster.Members[1]
	resp := Msg{
		Tp:   Rpc,
		From: newCandidate,
		To:   commCfg.cluster.Me,
		Payload: &RequestVoteReq{
			Term:        cand.currentTerm + 1,
			CandidateId: newCandidate,
		},
	}

	// when
	res := cand.TakeAction(resp)

	// then
	c.Assert(res.Tp, Equals, MoveState)
	if f, ok := res.Payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (resp.Payload.(*RequestVoteReq)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestCandidateWillBackToFollowerWhenReceiveAppendReqNewTerm(c *C) {
	// given
	cand := NewFollower(commCfg, mockSm).toCandidate(false)
	cand.currentTerm = 3

	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  4,
			PrevLogIndex: 5,
			Entries:      []Entry{{Term: 4, Idx: 6, Cmd: ""}},
		},
	}

	// when
	res := cand.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, MoveState)
	if f, ok := res.Payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (req.Payload.(*AppendEntriesReq)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestCandidateWillBackToFollowerWhenReceiveAppendReqCurrentTerm(c *C) {
	// given
	cand := NewFollower(commCfg, mockSm).toCandidate(false)
	cand.currentTerm = 3

	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         3,
			PrevLogTerm:  3,
			PrevLogIndex: 5,
			Entries:      []Entry{{Term: 3, Idx: 6, Cmd: ""}},
		},
	}

	// when
	res := cand.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, MoveState)
	if f, ok := res.Payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (req.Payload.(*AppendEntriesReq)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestCandidateShouldIgnoreAnyMsgThatTermOlderThanItself(c *C) {
	// given
	cand := NewFollower(commCfg, mockSm).toCandidate(false)
	cand.currentTerm = 2

	// when
	msg1 := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term: 1,
		},
	}
	res1 := cand.TakeAction(msg1)

	msg2 := Msg{
		Tp: Rpc,
		Payload: &RequestVoteResp{
			Term:        1,
			VoteGranted: true,
		},
	}
	res2 := cand.TakeAction(msg2)

	msg3 := Msg{
		Tp: Rpc,
		Payload: &RequestVoteReq{
			Term: 1,
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
	cand := NewFollower(commCfg, mockSm).toCandidate(false)
	cand.currentTerm = currTerm
	cand.cfg.electionTimeout = 3
	cand.cfg.tickCnt = 2

	req := Msg{Tp: Tick}

	// when
	res := cand.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.From, Equals, cand.cfg.cluster.Me)
	c.Assert(res.To, Equals, All)
	if rv, ok := res.Payload.(*RequestVoteReq); !ok {
		c.Fail()
	} else {
		c.Assert(rv.Term, Equals, currTerm+1)
		c.Assert(rv.CandidateId, Equals, cand.cfg.cluster.Me)
	}

	// re-random timeout
	legalElectionTimeout := cand.cfg.electionTimeout >= cand.cfg.electionTimeoutMin && cand.cfg.electionTimeout <= cand.cfg.electionTimeoutMax
	c.Assert(legalElectionTimeout, Equals, true)
}

func (t *T) TestCandidateForwardToLeaderWhenReceiveMajorityVotes(c *C) {
	// given
	voteFollowerId0 := commCfg.cluster.Members[1]
	voteFollowerId1 := commCfg.cluster.Members[3]

	cand := NewFollower(commCfg, mockSm).toCandidate(false)
	cand.currentTerm = 3
	cand.log = append(cand.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 3, Idx: 5, Cmd: ""})

	buildResp := func(id Id) Msg {
		return Msg{
			Tp:   Rpc,
			From: id,
			To:   commCfg.cluster.Me,
			Payload: &RequestVoteResp{
				Term:        3,
				VoteGranted: true,
			},
		}
	}

	// when
	_ = cand.TakeAction(buildResp(voteFollowerId0))
	res := cand.TakeAction(buildResp(voteFollowerId1))

	// then
	c.Assert(len(cand.voted), Equals, 2)
	c.Assert(res.Tp, Equals, MoveState)
	if l, ok := res.Payload.(*Leader); ok {
		lastLogIndex := cand.log[len(cand.log)-1].Idx
		for _, v := range cand.cfg.cluster.Members {
			if v == cand.cfg.cluster.Me {
				continue
			}
			c.Assert(l.nextIndex[v], Equals, lastLogIndex+1)
			c.Assert(l.matchIndex[v], Equals, InvalidIndex)
		}
	} else {
		c.Fail()
		c.Logf("Should move to leader.")
	}
}
