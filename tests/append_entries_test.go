package tests

import (
	"github.com/LENSHOOD/go-raft/core"
	. "gopkg.in/check.v1"
	"time"
)

func (t *T) TestHappyPathLogAppends(c *C) {
	r := newRouter()

	svr0 := newSvr(0)
	r.register(svr0)
	svr1 := newSvr(1)
	r.register(svr1)
	svr2 := newSvr(2)
	r.register(svr2)

	go r.run()

	var leader *svr
	timeout := waitCondition(func() bool {
		l, ok := getLeader([]*svr{svr0, svr1, svr2})
		leader = l
		return ok
	}, time.Second)
	if timeout {
		c.Errorf("No server turned to leader before time exceeded, test failed.")
		c.Fail()
	}

	r.exec(leader, "1", "a")
	r.exec(leader, "2", "b")
	r.exec(leader, "3", "c")

	// wait leader apply all
	for len(svr0.sm.cmds) < 3 || len(svr1.sm.cmds) < 3 || len(svr2.sm.cmds) < 3 {}

	expectationLog := []core.Entry{
		{Term: 1, Idx: 1, Cmd: "1"},
		{Term: 1, Idx: 2, Cmd: "2"},
		{Term: 1, Idx: 3, Cmd: "3"},
	}

	c.Assert(svr0.mgr.GetAllEntries(), DeepEquals, expectationLog)
	c.Assert(svr1.mgr.GetAllEntries(), DeepEquals, expectationLog)
	c.Assert(svr2.mgr.GetAllEntries(), DeepEquals, expectationLog)

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}

func (t *T) TestFollowerHoldThenLeaderWillNotReturnToCmdUntilFollowerRecover(c *C) {
	r := newRouter()

	svr0 := newSvr(0)
	r.register(svr0)
	svr1 := newSvr(1)
	r.register(svr1)
	svr2 := newSvr(2)
	r.register(svr2)

	go r.run()

	var leader *svr
	timeout := waitCondition(func() bool {
		l, ok := getLeader([]*svr{svr0, svr1, svr2})
		leader = l
		return ok
	}, time.Second)
	if timeout {
		c.Errorf("No server turned to leader before time exceeded, test failed.")
		c.Fail()
	}

	// hold all followers
	for _, f := range getFollowers([]*svr{svr0, svr1, svr2}) {
		r.hold(f)
	}

	r.exec(leader, "1", "a")
	r.exec(leader, "2", "b")
	r.exec(leader, "3", "c")

	time.Sleep(100*time.Millisecond)

	// resume all followers
	for _, f := range getFollowers([]*svr{svr0, svr1, svr2}) {
		c.Assert(len(f.sm.cmds), Equals, 0)
		r.resume(f)
	}

	// wait leader apply all
	for len(svr0.sm.cmds) < 3 || len(svr1.sm.cmds) < 3 || len(svr2.sm.cmds) < 3 {}

	expectationLog := []core.Entry{
		{Term: 1, Idx: 1, Cmd: "1"},
		{Term: 1, Idx: 2, Cmd: "2"},
		{Term: 1, Idx: 3, Cmd: "3"},
	}

	c.Assert(svr0.mgr.GetAllEntries(), DeepEquals, expectationLog)
	c.Assert(svr1.mgr.GetAllEntries(), DeepEquals, expectationLog)
	c.Assert(svr2.mgr.GetAllEntries(), DeepEquals, expectationLog)

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}

