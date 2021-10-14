package tests

import (
	"context"
	. "gopkg.in/check.v1"
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

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	defer cancelFunc()
	for !svr0.mgr.IsLeader() && !svr1.mgr.IsLeader() && !svr2.mgr.IsLeader() {
		select {
		case <-ctx.Done():
			c.Errorf("No server turned to leader before time exceeded, test failed.")
			c.Fail()
			break
		default:
		}
	}
}