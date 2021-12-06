package tests

import (
	"context"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	. "gopkg.in/check.v1"
	"strconv"
	"time"
)

func (t *T) TestHappyPathLeaderElection(c *C) {
	r := newRouter()

	svr0 := newSvr(0, 3)
	r.register(svr0)
	svr1 := newSvr(1, 3)
	r.register(svr1)
	svr2 := newSvr(2, 3)
	r.register(svr2)

	go r.run()

	// can elect a leader
	_ = waitLeader(c, []*svr{svr0, svr1, svr2})

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}

func (t *T) TestAllCandidateCanEventuallyBecomeLeaderOrFollower(c *C) {
	r := newRouter()

	// first hold all, to let them turn to candidate
	svr0 := newSvr(0, 3)
	r.register(svr0)
	r.hold(svr0)
	svr1 := newSvr(1, 3)
	r.register(svr1)
	r.hold(svr1)
	svr2 := newSvr(2, 3)
	r.register(svr2)
	r.hold(svr2)
	svrs := []*svr{svr0, svr1, svr2}

	go r.run()

	// wait to become candidate
	waitAllBecomeCandidate(c, svrs)

	r.resume(svr0)
	r.resume(svr1)
	r.resume(svr2)

	// can elect a leader
	_ = waitLeader(c, svrs)

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}

func (t *T) TestLeaderHoldWillLeadToNewLeaderElected(c *C) {
	r := newRouter()

	// first hold all, to let them turn to candidate
	svr0 := newSvr(0, 3)
	r.register(svr0)
	svr1 := newSvr(1, 3)
	r.register(svr1)
	svr2 := newSvr(2, 3)
	r.register(svr2)
	svrs := []*svr{svr0, svr1, svr2}

	go r.run()

	// wait to election
	oldLeader := waitLeader(c, svrs)

	r.hold(oldLeader)

	// can elect a new leader
	_ = waitLeaderWithException(c, svrs, oldLeader)

	r.resume(oldLeader)

	timeout := waitCondition(func() bool { return oldLeader.mgr.IsFollower() }, 5*time.Second)
	if timeout {
		c.Errorf("old leader should become follower, but not.")
		c.Fail()
	}

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}

func (t *T) TestShorterCommittedLogHolderCanNeverBeLeader(c *C) {
	r := newRouter()

	svr0 := newSvr(0, 3)
	r.register(svr0)
	svr1 := newSvr(1, 3)
	r.register(svr1)
	svr2 := newSvr(2, 3)
	r.register(svr2)
	svrs := []*svr{svr0, svr1, svr2}

	go r.run()

	// wait to election
	leader := waitLeader(c, svrs)
	followers := getFollowers(svrs)

	// 1. build different log
	// leader log: 0
	// follower0 log: (empty)
	// follower1 log: (empty)
	strOfI := strconv.Itoa(999)
	_ = leader.mgr.Dispatcher.RegisterResp(mgr.Address(strOfI))

	// hold one of followers
	r.hold(followers[1])

	// run command
	r.exec(leader, core.Command(strOfI), mgr.Address(strOfI))

	// let first entry committed
	waitNumOfSvrLogLength(c, svrs, 1, 2)

	// hold leader
	r.hold(leader)

	// 2. turn all into follower then turn to candidate
	for _, s := range svrs {
		s.inputCh <- &mgr.Rpc{
			Ctx:  context.TODO(),
			Addr: "",
			Payload: &core.RequestVoteReq{
				Term: 2,
				// leader transfer == true force follower turn to candidate
				LeaderTransfer: true,
			},
		}
	}

	waitAllBecomeCandidate(c, svrs)

	// 3. resume all to make election (shorter log svr resume first)
	for _, svr := range svrs {
		r.resume(svr)
	}

	newLeader := waitLeader(c, svrs)

	// the shorter log follower[1] will never become leader
	c.Assert(newLeader, Not(Equals), followers[1])

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}
