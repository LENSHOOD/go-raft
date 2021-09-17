package test

import (
	"go-raft/core"
	. "gopkg.in/check.v1"
	"testing"
)

// hook up go-check to go testing
func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

func (t *T) TestFollowerVoteWithInit(c *C) {
	// given
	cluster := core.Cluster{
		Me:     1,
		Others: []core.Id {2, 3},
	}

	req := &core.RequestVoteReq {
		Term:        1,
		CandidateId: 2,
	}

	f := core.NewFollower(cluster)

	// when
	obj, resp := f.TakeAction(req)

	// then
	voteResp := resp.(*core.RequestVoteResp)
	c.Assert(obj, Equals, f)
	c.Assert(voteResp.Term, Equals, core.Term(1))
	c.Assert(voteResp.VoteGranted, Equals, true)
}