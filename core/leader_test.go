package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestLeaderShouldSendHeartbeatEveryFixedTicks(c *C) {
	// given
	l := NewFollower(commCfg).toCandidate().toLeader()
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: ""}}

	heartbeatInterval = 3
	tick := Msg{tp: Tick}

	// when
	_ = l.TakeAction(tick)
	_ = l.TakeAction(tick)
	res := l.TakeAction(tick)

	// then
	c.Assert(res.tp, Equals, Req)
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
	l := NewFollower(commCfg).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}}

	cmdReqMsg := Msg{
		tp: Req,
		from: Id(999),
		to: l.cfg.leader,
		payload: &CmdReq{
			Cmd: "test",
		},
	}

	// when
	res := l.TakeAction(cmdReqMsg)

	// then msg
	c.Assert(res.tp, Equals, Req)
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
	c.Assert(len(l.entryCtxs), Equals, 1)
	c.Assert(l.entryCtxs[Index(3)].clientId, Equals, Id(999))
}