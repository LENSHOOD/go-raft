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
