package core

import (
	. "gopkg.in/check.v1"
	"testing"
)

// hook up go-check to go testing
func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

func (t *T) TestFollowerVoteWithInit(c *C) {
	// given
	cluster := Cluster {
		Me:     1,
		Others: []Id{2, 3},
	}

	req := &RequestVoteReq {
		Term:        1,
		CandidateId: 2,
	}

	f := NewFollower(cluster)

	// when
	obj, resp := f.TakeAction(req)

	// then
	voteResp := resp.(*RequestVoteResp)
	c.Assert(obj, Equals, f)
	c.Assert(voteResp.Term, Equals, Term(1))
	c.Assert(voteResp.VoteGranted, Equals, true)
}

func (t *T) TestFollowerNotVoteWhenCandidateHoldSmallerTerms(c *C) {
	// given
	cluster := Cluster {
		Me:     1,
		Others: []Id{2, 3},
	}

	req := &RequestVoteReq {
		Term:        1,
		CandidateId: 2,
	}

	f := NewFollower(cluster)
	f.currentTerm = 2

	// when
	obj, resp := f.TakeAction(req)

	// then
	voteResp := resp.(*RequestVoteResp)
	c.Assert(obj, Equals, f)
	c.Assert(voteResp.Term, Equals, Term(2))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerNotVoteWhenAlreadyVotedToAnother(c *C) {
	// given
	cluster := Cluster {
		Me:     1,
		Others: []Id{2, 3},
	}

	req := &RequestVoteReq {
		Term:        1,
		CandidateId: 2,
	}

	f := NewFollower(cluster)
	f.currentTerm = 1
	f.votedFor = 3

	// when
	obj, resp := f.TakeAction(req)

	// then
	voteResp := resp.(*RequestVoteResp)
	c.Assert(obj, Equals, f)
	c.Assert(voteResp.Term, Equals, Term(1))
	c.Assert(voteResp.VoteGranted, Equals, false)
}