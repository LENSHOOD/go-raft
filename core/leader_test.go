package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestLeaderShouldSendHeartbeatEveryFixedTicks(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: ""}}

	heartbeatInterval = 3
	tick := Msg{tp: Tick}

	// when
	_ = l.TakeAction(tick)
	_ = l.TakeAction(tick)
	res := l.TakeAction(tick)

	// then
	c.Assert(res.tp, Equals, Rpc)
	c.Assert(res.to, Equals, All)
	if req, ok := res.payload.(*AppendEntriesReq); ok {
		c.Assert(len(req.Entries), Equals, 0)
	} else {
		c.Fail()
		c.Logf("Heartbeat req should be AppendEntriesReq")
	}
}

func (t *T) TestLeaderShouldSendAppendLogToEveryFollower(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}}

	cmdReqMsg := Msg{
		tp:   Cmd,
		from: Id(999),
		to:   l.cfg.leader,
		payload: &CmdReq{
			Cmd: "test",
		},
	}

	// when
	res := l.TakeAction(cmdReqMsg)

	// then msg
	c.Assert(res.tp, Equals, Rpc)
	c.Assert(res.to, Equals, All)

	// then payload
	if appendLogReq, ok := res.payload.(*AppendEntriesReq); ok {
		c.Assert(appendLogReq.Term, Equals, l.currentTerm)
		c.Assert(appendLogReq.LeaderId, Equals, l.cfg.leader)
		c.Assert(appendLogReq.PrevLogTerm, Equals, Term(1))
		c.Assert(appendLogReq.PrevLogIndex, Equals, Index(2))
		c.Assert(appendLogReq.Entries, DeepEquals, []Entry{{Term: 1, Idx: 3, Cmd: "test"}})
	} else {
		c.Fail()
		c.Logf("Payload should be AppendEntriesReq")
	}

	// then client context
	c.Assert(len(l.clientCtxs), Equals, 1)
	c.Assert(l.clientCtxs[Index(3)].clientId, Equals, Id(999))
}

func (t *T) TestLeaderShouldIncrementMatchIndexWhenReceiveSuccessRespFromFollower(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}}

	l.matchIndex[Id(2)] = 0

	resp := Msg{
		tp:   Rpc,
		from: Id(2),
		to:   l.cfg.leader,
		payload: &AppendEntriesResp{
			Term:    1,
			Success: true,
		},
	}

	// when recv two success resp
	_ = l.TakeAction(resp)
	_ = l.TakeAction(resp)

	// then
	c.Assert(l.matchIndex[Id(2)], Equals, Index(2))
}

func (t *T) TestLeaderShouldIncrementCommittedIndexAndResponseToClientWhenReceiveMajoritySuccesses(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}}

	l.matchIndex[Id(2)] = 1
	l.matchIndex[Id(3)] = 1
	l.matchIndex[Id(4)] = 0
	l.matchIndex[Id(5)] = 1
	l.clientCtxs[Index(2)] = clientCtx{clientId: Id(999)}

	buildResp := func(id Id) Msg {
		return Msg{
			tp:   Rpc,
			from: id,
			to:   l.cfg.leader,
			payload: &AppendEntriesResp{
				Term:    1,
				Success: true,
			},
		}
	}

	// when
	res1 := l.TakeAction(buildResp(2))
	res2 := l.TakeAction(buildResp(3))

	// then
	c.Assert(l.matchIndex[Id(2)], Equals, Index(2))
	c.Assert(l.matchIndex[Id(3)], Equals, Index(2))
	c.Assert(l.matchIndex[Id(4)], Equals, Index(0))
	c.Assert(l.matchIndex[Id(5)], Equals, Index(1))

	c.Assert(res1, Equals, NullMsg)
	if payload, ok := res2.payload.(*CmdResp); ok {
		c.Assert(payload.Success, Equals, true)
		c.Assert(res2.to, Equals, Id(999))

		_, exist := l.clientCtxs[Index(2)]
		c.Assert(exist, Equals, false)

		c.Assert(l.commitIndex, Equals, Index(2))
	} else {
		c.Fail()
		c.Logf("Should get CmdResp")
	}
}

func (t *T) TestLeaderWillBackToFollowerWhenReceiveAnyRpcWithNewTerm(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 3

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
	res := l.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, MoveState)
	if f, ok := res.payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (req.payload.(*AppendEntriesReq)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestLeaderShouldDecreaseNextIndexWhenReceiveFailureRespFromFollower(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}}

	fid := Id(2)
	l.nextIndex[fid] = 4

	resp := Msg{
		tp:   Rpc,
		from: fid,
		to:   l.cfg.leader,
		payload: &AppendEntriesResp{
			Term:    1,
			Success: false,
		},
	}

	// when
	res := l.TakeAction(resp)

	// then
	c.Assert(res.tp, Equals, Rpc)
	c.Assert(res.to, Equals, fid)
	if req, ok := res.payload.(*AppendEntriesReq); ok {
		c.Assert(req.Term, Equals, l.currentTerm)
		// nextIndex--
		c.Assert(l.nextIndex[fid], Equals, Index(3))
		c.Assert(req.PrevLogTerm, Equals, Term(1))
		c.Assert(req.PrevLogIndex, Equals, Index(1))
		// now the entries contain more than one
		c.Assert(req.Entries, DeepEquals, []Entry{{Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}})
	} else {
		c.Fail()
		c.Logf("Should get AppendEntriesReq")
	}
}

func (t *T) TestLeaderShouldKeepDecreaseNextIndexUntilFirstEntryWhenReceiveFailureRespFromFollower(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}}

	fid := Id(2)
	l.nextIndex[fid] = 4

	resp := Msg{
		tp:   Rpc,
		from: fid,
		to:   l.cfg.leader,
		payload: &AppendEntriesResp{
			Term:    1,
			Success: false,
		},
	}

	// when
	_ = l.TakeAction(resp)
	_ = l.TakeAction(resp)
	res := l.TakeAction(resp)

	// then
	c.Assert(res.tp, Equals, Rpc)
	c.Assert(res.to, Equals, fid)
	if req, ok := res.payload.(*AppendEntriesReq); ok {
		c.Assert(req.Term, Equals, l.currentTerm)
		// nextIndex--
		c.Assert(l.nextIndex[fid], Equals, Index(1))
		c.Assert(req.PrevLogTerm, Equals, InvalidTerm)
		c.Assert(req.PrevLogIndex, Equals, InvalidIndex)
		// now the entries contain more than one
		c.Assert(req.Entries, DeepEquals, []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}})
	} else {
		c.Fail()
		c.Logf("Should get AppendEntriesReq")
	}
}
