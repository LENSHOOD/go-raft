package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestFollowerVoteWithInit(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &RequestVoteReq{
			Term:        1,
			CandidateId: 2,
		},
	}

	f := NewFollower(cfg)

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	voteResp := res.payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(1))
	c.Assert(voteResp.VoteGranted, Equals, true)
	c.Assert(f.votedFor, Equals, Id(2))
}

func (t *T) TestFollowerNotVoteWhenCandidateHoldSmallerTerms(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &RequestVoteReq{
			Term:        1,
			CandidateId: 2,
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 2

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	voteResp := res.payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(2))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerNotVoteWhenAlreadyVotedToAnother(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &RequestVoteReq{
			Term:        1,
			CandidateId: 2,
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 1
	f.votedFor = 3

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	voteResp := res.payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(1))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerReVoteWhenBiggerTermReceived(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &RequestVoteReq{
			Term:        2,
			CandidateId: 3,
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 1
	f.votedFor = 2

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	voteResp := res.payload.(*RequestVoteResp)
	c.Assert(f.votedFor, Equals, Id(3))
	c.Assert(voteResp.Term, Equals, Term(2))
	c.Assert(voteResp.VoteGranted, Equals, true)
}

func (t *T) TestFollowerNotVoteWhenLastEntryTermBiggerThanCandidate(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &RequestVoteReq{
			Term:         5,
			CandidateId:  2,
			LastLogIndex: 1,
			LastLogTerm:  2,
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 4
	f.log = append(f.log, Entry{
		Term: 3,
		Idx:  5,
		Cmd:  "",
	})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	voteResp := res.payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(5))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerNotVoteWhenLastEntryTermSameAsCandidateButIndexMore(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &RequestVoteReq{
			Term:         5,
			CandidateId:  2,
			LastLogIndex: 1,
			LastLogTerm:  2,
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 4
	f.log = append(f.log, Entry{Term: 2, Idx: 1, Cmd: "1"}, Entry{Term: 2, Idx: 2, Cmd: "2"}, Entry{Term: 2, Idx: 3, Cmd: "3"})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	voteResp := res.payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(5))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerNotAppendLogWhenLeaderTermLessThanCurrTerm(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &AppendEntriesReq{
			Term: 1,
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 2

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	appendResp := res.payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(2))
	c.Assert(appendResp.Success, Equals, false)
}

func (t *T) TestFollowerNotAppendLogWhenPrevTermNotMatch(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &AppendEntriesReq{
			Term:        4,
			PrevLogTerm: 2,
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	appendResp := res.payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, false)
}

func (t *T) TestFollowerNotAppendLogWhenPrevTermMatchButPrevIndexNotMatch(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  3,
			PrevLogIndex: 5,
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	appendResp := res.payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, false)
}

func (t *T) TestFollowerAppendLogToLast(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  4,
			PrevLogIndex: 5,
			Entries:      []Entry{{Term: 4, Idx: 6, Cmd: ""}},
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	appendResp := res.payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, true)
	c.Assert(f.log[len(f.log)-1], Equals, Entry{Term: 4, Idx: 6, Cmd: ""})
}

func (t *T) TestFollowerAppendLogToRightIdxAndRemoveTheFollowEntriesThenUpdateCommitIndexToLeaderCommit(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  1,
			PrevLogIndex: 1,
			LeaderCommit: 3,
			Entries:      []Entry{{Term: 2, Idx: 2, Cmd: ""}, {Term: 2, Idx: 3, Cmd: ""}, {Term: 2, Idx: 4, Cmd: ""}},
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	appendResp := res.payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, true)

	expectedLog := []Entry{{Term: 1, Idx: 1, Cmd: ""}, {Term: 2, Idx: 2, Cmd: ""}, {Term: 2, Idx: 3, Cmd: ""}, {Term: 2, Idx: 4, Cmd: ""}}
	c.Assert(expectedLog, DeepEquals, f.log)
	c.Assert(f.commitIndex, Equals, Index(3))
	c.Assert(f.lastApplied, Equals, Index(3))
}

func (t *T) TestFollowerAppendLogToRightIdxAndRemoveTheFollowEntriesNotSameThenUpdateCommitIndexToLastNewEntry(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
	}

	req := Msg{
		tp: Req,
		payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  1,
			PrevLogIndex: 1,
			LeaderCommit: 8,
			Entries:      []Entry{{Term: 3, Idx: 3, Cmd: ""}, {Term: 3, Idx: 4, Cmd: ""}, {Term: 4, Idx: 5, Cmd: ""}},
		},
	}

	f := NewFollower(cfg)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 3, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, Resp)
	appendResp := res.payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, true)

	expectedLog := []Entry{{Term: 1, Idx: 1, Cmd: ""}, {Term: 3, Idx: 3, Cmd: ""}, {Term: 3, Idx: 4, Cmd: ""}, {Term: 4, Idx: 5, Cmd: ""}}
	c.Assert(expectedLog, DeepEquals, f.log)
	c.Assert(f.commitIndex, Equals, Index(5))
	c.Assert(f.lastApplied, Equals, Index(5))
}

func (t *T) TestFollowerTriggerElectionTimeoutWithEmptyTick(c *C) {
	// given
	cfg := Config{
		cluster: Cluster{
			Me:     1,
			Others: []Id{2, 3},
		},
		electionTimeoutMin: 3,
		electionTimeoutMax: 10,
		electionTimeout:    3,
		tickCnt:            0,
	}

	req := Msg{tp: Tick}

	f := NewFollower(cfg)

	// when
	_ = f.TakeAction(req)
	_ = f.TakeAction(req)
	res := f.TakeAction(req)

	// then
	c.Assert(res.tp, Equals, MoveState)
	c.Assert(res.payload, Not(Equals), f)
	if candidate, ok := res.payload.(*Candidate); !ok {
		c.Fail()
	} else {
		c.Assert(candidate.log, DeepEquals, f.log)
		c.Assert(candidate.votedFor, Equals, f.cfg.cluster.Me)
		c.Assert(candidate.cfg.tickCnt, Not(Equals), int64(0))
		legalElectionTimeout := candidate.cfg.electionTimeout >= candidate.cfg.electionTimeoutMin && candidate.cfg.electionTimeout <= candidate.cfg.electionTimeoutMax
		c.Assert(legalElectionTimeout, Equals, true)
	}
}
