package tests

import (
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	. "gopkg.in/check.v1"
	"strconv"
	"time"
)

func (t *T) TestHappyPathLeaderElection(c *C) {
	r := newRouter()

	svr0 := newSvr(0)
	r.register(svr0)
	svr1 := newSvr(1)
	r.register(svr1)
	svr2 := newSvr(2)
	r.register(svr2)

	go r.run()

	timeout := waitCondition(func() bool { return svr0.mgr.IsLeader() || svr1.mgr.IsLeader() || svr2.mgr.IsLeader() }, time.Second)
	if timeout {
		c.Errorf("No server turned to leader before time exceeded, test failed.")
		c.Fail()
	}

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}

func (t *T) TestAllCandidateCanEventuallyBecomeLeaderOrFollower(c *C) {
	r := newRouter()

	// first hold all, to let them turn to candidate
	svr0 := newSvr(0)
	r.register(svr0)
	r.hold(svr0)
	svr1 := newSvr(1)
	r.register(svr1)
	r.hold(svr1)
	svr2 := newSvr(2)
	r.register(svr2)
	r.hold(svr2)

	go r.run()

	// wait to become candidate
	for !svr0.mgr.IsCandidate() || !svr1.mgr.IsCandidate() || !svr2.mgr.IsCandidate() {
	}

	r.resume(svr0)
	r.resume(svr1)
	r.resume(svr2)

	timeout := waitCondition(func() bool { return svr0.mgr.IsLeader() || svr1.mgr.IsLeader() || svr2.mgr.IsLeader() }, time.Second)
	if timeout {
		c.Errorf("No server turned to leader before time exceeded, test failed.")
		c.Fail()
	}

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}

func (t *T) TestLeaderHoldWillLeadToNewLeaderElected(c *C) {
	r := newRouter()

	// first hold all, to let them turn to candidate
	svr0 := newSvr(0)
	r.register(svr0)
	svr1 := newSvr(1)
	r.register(svr1)
	svr2 := newSvr(2)
	r.register(svr2)

	go r.run()

	var oldLeader *svr
	// wait to election
	for {
		if leader, ok := getLeader([]*svr{svr0, svr1, svr2}); ok {
			oldLeader = leader
			break
		}
	}

	r.hold(oldLeader)

	timeout := waitCondition(func() bool {
		return (svr0 != oldLeader && svr0.mgr.IsLeader()) ||
			(svr1 != oldLeader && svr1.mgr.IsLeader()) ||
			(svr2 != oldLeader && svr2.mgr.IsLeader())
	}, 5*time.Second)
	if timeout {
		c.Errorf("No server turned to leader before time exceeded, test failed.")
		c.Fail()
		return
	}

	r.resume(oldLeader)

	timeout = waitCondition(func() bool { return oldLeader.mgr.IsFollower() }, 5*time.Second)
	if timeout {
		c.Errorf("old leader should become follower, but not.")
		c.Fail()
	}

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}

func (t *T) TestShorterLogHolderCanNeverBeLeader(c *C) {
	r := newRouter()

	svr0 := newSvr(0)
	r.register(svr0)
	svr1 := newSvr(1)
	r.register(svr1)
	svr2 := newSvr(2)
	r.register(svr2)

	go r.run()

	// wait to election
	svrs := []*svr{svr0, svr1, svr2}
	var leader *svr
	for {
		if l, ok := getLeader([]*svr{svr0, svr1, svr2}); ok {
			leader = l
			break
		}
	}
	followers := getFollowers(svrs)

	// 1. build different log
	// leader log: 0
	// follower0 log: (empty)
	// follower1 log: (empty)
	strOfI := strconv.Itoa(999)
	_ = leader.mgr.Dispatcher.RegisterResp(mgr.Address(strOfI))
	r.exec(leader, core.Command(strOfI), mgr.Address(strOfI))

	r.hold(followers[0])
	r.hold(followers[1])
	r.hold(leader)

	// 2. turn all into follower then turn to candidate
	for _, s := range svrs {
		s.inputCh <- &mgr.Rpc{
			Addr: "",
			Payload: &core.RequestVoteReq{
				Term: 2,
			},
		}
	}

	timeout := waitCondition(func() bool { return svr0.mgr.IsCandidate() && svr1.mgr.IsCandidate() && svr2.mgr.IsCandidate() }, 5*time.Second)
	if timeout {
		c.Errorf("No server turned to follower before time exceeded, test failed.")
		c.Fail()
		return
	}

	// 3. resume all to make election (shorter log svr resume first)
	r.resume(followers[0])
	r.resume(followers[1])
	r.resume(leader)

	timeout = waitCondition(func() bool { return leader.mgr.IsLeader() }, 5*time.Second)
	if timeout {
		c.Errorf("leader should still be original leader, but not.")
		c.Fail()
	}

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
}
