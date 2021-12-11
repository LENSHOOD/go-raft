package tests

import (
	"github.com/LENSHOOD/go-raft/mgr"
	. "gopkg.in/check.v1"
	"time"
)

func (t *T) TestAddServerThenRemoveServer(c *C) {
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

	r.pause()
	// build new svr to be added
	svr3 := newSvr(3, 0)
	r.add(svr3)
	go svr3.mgr.Run()
	r.rerun()

	r.changeSvr(leader, &mgr.ConfigChange{Op: mgr.Add, Server: svr3.addr}, "c")

	// wait leader apply all, wait another 1s to let leader commit config change entry
	svrsOf4 := append(svrs, svr3)
	waitNumOfSvrLogLength(c, svrsOf4, 3, 4)
	time.Sleep(time.Second)

	c.Assert(svr0.mgr.GetConfig().Others, DeepEquals, []mgr.Address{svr1.addr, svr2.addr, svr3.addr})
	c.Assert(svr1.mgr.GetConfig().Others, DeepEquals, []mgr.Address{svr0.addr, svr2.addr, svr3.addr})
	c.Assert(svr2.mgr.GetConfig().Others, DeepEquals, []mgr.Address{svr0.addr, svr1.addr, svr3.addr})
	c.Assert(svr3.mgr.GetConfig().Others, DeepEquals, []mgr.Address{svr0.addr, svr1.addr, svr2.addr})

	// remove a follower
	followerToBeRemoved := getFollowers(svrsOf4)[0]
	r.changeSvr(leader, &mgr.ConfigChange{Op: mgr.Remove, Server: followerToBeRemoved.addr}, "d")

	// wait leader apply all
	waitNumOfSvrLogLength(c, svrsOf4, 4, 3)

	for _, svr := range svrsOf4 {
		if svr != followerToBeRemoved {
			c.Assert(len(svr.mgr.GetConfig().Others), Equals, 2)
		} else {
			// removed server will never receive any logs
			c.Assert(len(svr.mgr.GetConfig().Others), Equals, 3)
		}
	}

	// wait removed server turn to candidate
	waitCandidate(c, followerToBeRemoved)

	// disrupt follower cannot break cluster
	newClusterLeader := waitLeader(c, svrsOf4)
	c.Assert(newClusterLeader, Equals, leader)

	close(r.done)
	svr0.mgr.Stop()
	svr1.mgr.Stop()
	svr2.mgr.Stop()
	svr3.mgr.Stop()
}
