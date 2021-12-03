package tests

import (
	"github.com/LENSHOOD/go-raft/core"
	. "gopkg.in/check.v1"
	"time"
)

func (t *T) TestHappyPathLogAppends(c *C) {
	r := newRouter()

	svr0 := newSvr(0, 3)
	r.register(svr0)
	svr1 := newSvr(1, 3)
	r.register(svr1)
	svr2 := newSvr(2, 3)
	r.register(svr2)
	svrs := []*svr{svr0, svr1, svr2}

	go r.run()

	leader := waitLeader(c, svrs)

	r.exec(leader, "1", "a")
	r.exec(leader, "2", "b")
	r.exec(leader, "3", "c")

	// wait leader apply all
	waitNumOfSvrLogLength(c, svrs, 3, 3)

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

	svr0 := newSvr(0, 3)
	r.register(svr0)
	svr1 := newSvr(1, 3)
	r.register(svr1)
	svr2 := newSvr(2, 3)
	r.register(svr2)
	svrs := []*svr{svr0, svr1, svr2}

	go r.run()

	leader := waitLeader(c, svrs)

	// hold all followers
	for _, f := range getFollowers(svrs) {
		r.hold(f)
	}

	r.exec(leader, "1", "a")
	r.exec(leader, "2", "b")
	r.exec(leader, "3", "c")

	time.Sleep(100 * time.Millisecond)

	// resume all followers
	for _, f := range getFollowers(svrs) {
		c.Assert(len(f.sm.cmds), Equals, 0)
		r.resume(f)
	}

	// wait leader apply all
	waitNumOfSvrLogLength(c, svrs, 3, 3)

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

func (t *T) TestClusterCanFinallyReachConsistentAfterLeaderAndFollowerHold(c *C) {
	r := newRouter()

	svr0 := newSvr(0, 5)
	r.register(svr0)
	svr1 := newSvr(1, 5)
	r.register(svr1)
	svr2 := newSvr(2, 5)
	r.register(svr2)
	svr3 := newSvr(3, 5)
	r.register(svr3)
	svr4 := newSvr(4, 5)
	r.register(svr4)

	svrs := []*svr{svr0, svr1, svr2, svr3, svr4}
	go r.run()

	leader := waitLeader(c, svrs)
	followers := getFollowers(svrs)

	// hold first follower
	r.hold(followers[0])

	// exec cmd
	r.exec(leader, "1", "a")
	waitNumOfSvrLogLength(c, svrs, 1, 4)

	// hold second follower
	r.hold(followers[1])

	// exec cmd
	r.exec(leader, "2", "b")
	waitNumOfSvrLogLength(c, svrs, 2, 3)

	// hold leader then resume first follower
	r.hold(leader)
	r.resume(followers[0])

	// wait new leader
	leader2 := waitLeaderWithException(c, svrs, leader)
	followers = getFollowers(svrs)

	// hold first follower and resume old leader
	r.hold(followers[0])
	r.resume(leader)

	// exec cmd
	r.exec(leader2, "3", "c")
	waitNumOfSvrLogLength(c, svrs, 3, 3)

	// hold all followers then cluster down
	for _, f := range followers {
		r.hold(f)
	}

	// exec cmd will make not be processed
	r.exec(leader2, "4", "d")
	waitNumOfSvrLogLength(c, svrs, 4, 1)

	// resume all
	for _, f := range svrs {
		r.resume(f)
	}
	waitNumOfSvrLogLength(c, svrs, 4, 5)

	// hold leader2
	r.hold(leader2)

	// wait election new leader
	leader3 := waitLeaderWithException(c, svrs, leader2)

	// exec cmd
	r.exec(leader3, "5", "e")

	// wait leader apply all
	waitNumOfSvrLogLength(c, svrs, 4, 4)

	// resume leader2
	r.resume(leader2)
	waitNumOfSvrLogLength(c, svrs, 5, 5)

	expectationLog := []core.Entry{
		{Term: 1, Idx: 1, Cmd: "1"},
		{Term: 1, Idx: 2, Cmd: "2"},
		{Term: 2, Idx: 3, Cmd: "3"},
		{Term: 2, Idx: 4, Cmd: "4"},
		{Term: 3, Idx: 5, Cmd: "5"},
	}

	c.Assert(svr0.mgr.GetAllEntries(), DeepEquals, expectationLog)
	c.Assert(svr1.mgr.GetAllEntries(), DeepEquals, expectationLog)
	c.Assert(svr2.mgr.GetAllEntries(), DeepEquals, expectationLog)
	c.Assert(svr3.mgr.GetAllEntries(), DeepEquals, expectationLog)
	c.Assert(svr4.mgr.GetAllEntries(), DeepEquals, expectationLog)

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
	svr3.mgr.Stop()
	svr4.mgr.Stop()
}
