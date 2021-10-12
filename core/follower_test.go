package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestFollowerVoteWithInit(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &RequestVoteReq{
			Term:        1,
			CandidateId: 2,
		},
	}

	f := NewFollower(commCfg, mockSm)

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	voteResp := res.Payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(1))
	c.Assert(voteResp.VoteGranted, Equals, true)
	c.Assert(f.votedFor, Equals, Id(2))
}

func (t *T) TestFollowerNotVoteWhenCandidateHoldSmallerTerms(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &RequestVoteReq{
			Term:        1,
			CandidateId: 2,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 2

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	voteResp := res.Payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(2))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerNotVoteWhenAlreadyVotedToAnother(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &RequestVoteReq{
			Term:        1,
			CandidateId: 2,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 1
	f.votedFor = 3

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	voteResp := res.Payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(1))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerReVoteWhenBiggerTermReceived(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &RequestVoteReq{
			Term:        2,
			CandidateId: 3,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 1
	f.votedFor = 2

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	voteResp := res.Payload.(*RequestVoteResp)
	c.Assert(f.votedFor, Equals, Id(3))
	c.Assert(voteResp.Term, Equals, Term(2))
	c.Assert(voteResp.VoteGranted, Equals, true)
}

func (t *T) TestFollowerNotVoteWhenLastEntryTermBiggerThanCandidate(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &RequestVoteReq{
			Term:         5,
			CandidateId:  2,
			LastLogIndex: 1,
			LastLogTerm:  2,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4
	f.log = append(f.log, Entry{
		Term: 3,
		Idx:  5,
		Cmd:  "",
	})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	voteResp := res.Payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(5))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerNotVoteWhenLastEntryTermSameAsCandidateButIndexMore(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &RequestVoteReq{
			Term:         5,
			CandidateId:  2,
			LastLogIndex: 1,
			LastLogTerm:  2,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4
	f.log = append(f.log, Entry{Term: 2, Idx: 1, Cmd: "1"}, Entry{Term: 2, Idx: 2, Cmd: "2"}, Entry{Term: 2, Idx: 3, Cmd: "3"})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	voteResp := res.Payload.(*RequestVoteResp)
	c.Assert(voteResp.Term, Equals, Term(5))
	c.Assert(voteResp.VoteGranted, Equals, false)
}

func (t *T) TestFollowerNotAppendLogWhenLeaderTermLessThanCurrTerm(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term: 1,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 2

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(2))
	c.Assert(appendResp.Success, Equals, false)
}

func (t *T) TestFollowerNotAppendLogWhenPrevTermNotMatch(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  2,
			PrevLogIndex: 2,
			Entries:      []Entry{{Term: 3, Idx: 3, Cmd: ""}, {Term: 3, Idx: 4, Cmd: ""}, {Term: 4, Idx: 5, Cmd: ""}},
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, false)
}

func (t *T) TestFollowerNotAppendLogWhenPrevTermMatchButPrevIndexNotMatch(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  3,
			PrevLogIndex: 5,
			Entries:      []Entry{{Term: 4, Idx: 6, Cmd: ""}},
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, false)
}

func (t *T) TestFollowerReturnTrueButNotAppendLogWhenReceiveHeartbeatMsg(c *C) {
	// given
	heartbeat := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  InvalidTerm,
			PrevLogIndex: InvalidIndex,
			Entries:      []Entry{},
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	originalLog := append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})
	f.log = originalLog

	// when
	res := f.TakeAction(heartbeat)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, true)
	c.Assert(f.log, DeepEquals, originalLog)
}

func (t *T) TestFollowerShouldApplyLogWhenReceiveHeartbeatMsgContainsNewCommittedIdx(c *C) {
	// given
	heartbeat := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  InvalidTerm,
			PrevLogIndex: InvalidIndex,
			Entries:      []Entry{},
			LeaderCommit: 2,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4
	f.lastApplied = 1
	f.commitIndex = 1

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	originalLog := append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""}, Entry{Term: 1, Idx: 2, Cmd: ""})
	f.log = originalLog

	// when
	res := f.TakeAction(heartbeat)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, true)
	c.Assert(f.log, DeepEquals, originalLog)
	c.Assert(f.lastApplied, Equals, Index(2))
}

func (t *T) TestFollowerCanAppendFirstLog(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         1,
			PrevLogTerm:  0,
			PrevLogIndex: 0,
			Entries:      []Entry{{Term: 1, Idx: 1, Cmd: ""}},
			LeaderCommit: 0,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 1

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(1))
	c.Assert(appendResp.Success, Equals, true)
	c.Assert(f.log[len(f.log)-1], Equals, Entry{Term: 1, Idx: 1, Cmd: ""})
}

func (t *T) TestFollowerAppendLogToLast(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  4,
			PrevLogIndex: 5,
			Entries:      []Entry{{Term: 4, Idx: 6, Cmd: ""}},
			LeaderCommit: 5,
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, true)
	c.Assert(f.log[len(f.log)-1], Equals, Entry{Term: 4, Idx: 6, Cmd: ""})
}

func (t *T) TestFollowerAppendLogToRightIdxAndRemoveTheFollowEntriesThenUpdateCommitIndexToLeaderCommit(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  1,
			PrevLogIndex: 1,
			LeaderCommit: 3,
			Entries:      []Entry{{Term: 2, Idx: 2, Cmd: ""}, {Term: 2, Idx: 3, Cmd: ""}, {Term: 2, Idx: 4, Cmd: ""}},
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 4, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, true)

	expectedLog := []Entry{{Term: 1, Idx: 1, Cmd: ""}, {Term: 2, Idx: 2, Cmd: ""}, {Term: 2, Idx: 3, Cmd: ""}, {Term: 2, Idx: 4, Cmd: ""}}
	c.Assert(expectedLog, DeepEquals, f.log)
	c.Assert(f.commitIndex, Equals, Index(3))
	c.Assert(f.lastApplied, Equals, Index(3))
}

func (t *T) TestFollowerAppendLogToRightIdxAndRemoveTheFollowEntriesNotSameThenUpdateCommitIndexToLastNewEntry(c *C) {
	// given
	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  1,
			PrevLogIndex: 1,
			LeaderCommit: 8,
			Entries:      []Entry{{Term: 3, Idx: 3, Cmd: ""}, {Term: 3, Idx: 4, Cmd: ""}, {Term: 4, Idx: 5, Cmd: ""}},
		},
	}

	f := NewFollower(commCfg, mockSm)
	f.currentTerm = 4

	// Log (term:idx): 1:1 1:2 3:3 3:4 4:5
	f.log = append(f.log, Entry{Term: 1, Idx: 1, Cmd: ""},
		Entry{Term: 3, Idx: 3, Cmd: ""}, Entry{Term: 3, Idx: 4, Cmd: ""},
		Entry{Term: 3, Idx: 5, Cmd: ""})

	// when
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	appendResp := res.Payload.(*AppendEntriesResp)
	c.Assert(appendResp.Term, Equals, Term(4))
	c.Assert(appendResp.Success, Equals, true)

	expectedLog := []Entry{{Term: 1, Idx: 1, Cmd: ""}, {Term: 3, Idx: 3, Cmd: ""}, {Term: 3, Idx: 4, Cmd: ""}, {Term: 4, Idx: 5, Cmd: ""}}
	c.Assert(expectedLog, DeepEquals, f.log)
	c.Assert(f.commitIndex, Equals, Index(5))
	c.Assert(f.lastApplied, Equals, Index(5))
}

func (t *T) TestFollowerTriggerElectionTimeoutWithEmptyTick(c *C) {
	// given
	req := Msg{Tp: Tick}

	f := NewFollower(commCfg, mockSm)

	// when
	_ = f.TakeAction(req)
	_ = f.TakeAction(req)
	res := f.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, MoveState)
	c.Assert(res.Payload, Not(Equals), f)
	if candidate, ok := res.Payload.(*Candidate); !ok {
		c.Fail()
	} else {
		c.Assert(candidate.log, DeepEquals, f.log)
		c.Assert(candidate.votedFor, Equals, f.cfg.cluster.Me)
		c.Assert(candidate.cfg.tickCnt, Not(Equals), int64(0))
		legalElectionTimeout := candidate.cfg.electionTimeout >= candidate.cfg.electionTimeoutMin && candidate.cfg.electionTimeout <= candidate.cfg.electionTimeoutMax
		c.Assert(legalElectionTimeout, Equals, true)
	}
}
