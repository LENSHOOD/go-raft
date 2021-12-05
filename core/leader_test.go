package core

import (
	. "gopkg.in/check.v1"
)

func (t *T) TestLeaderShouldSendHeartbeatEveryFixedTicks(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: ""}}

	heartbeatDivideFactor = 3
	tick := Msg{Tp: Tick}

	// when
	_ = l.TakeAction(tick)
	_ = l.TakeAction(tick)
	res := l.TakeAction(tick)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, All)
	if req, ok := res.Payload.(*AppendEntriesReq); ok {
		lastEntry := l.getLastEntry()
		c.Assert(req.PrevLogIndex, Equals, lastEntry.Idx)
		c.Assert(req.PrevLogTerm, Equals, lastEntry.Term)
		c.Assert(len(req.Entries), Equals, 0)
	} else {
		c.Fail()
		c.Logf("Heartbeat req should be AppendEntriesReq")
	}
}

func (t *T) TestLeaderShouldSendAppendLogToEveryFollower(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}}

	cmdReqMsg := Msg{
		Tp:   Cmd,
		From: Id(999),
		To:   l.cfg.leader,
		Payload: &CmdReq{
			Cmd: "test",
		},
	}

	// when
	res := l.TakeAction(cmdReqMsg)

	// then msg
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, All)

	// then payload
	if appendLogReq, ok := res.Payload.(*AppendEntriesReq); ok {
		c.Assert(appendLogReq.Term, Equals, l.currentTerm)
		c.Assert(appendLogReq.LeaderId, Equals, l.cfg.leader)
		c.Assert(appendLogReq.PrevLogTerm, Equals, Term(1))
		c.Assert(appendLogReq.PrevLogIndex, Equals, Index(2))
		c.Assert(appendLogReq.Entries, DeepEquals, []Entry{{Term: 1, Idx: 3, Cmd: "test"}})
	} else {
		c.Fail()
		c.Logf("Payload should be AppendEntriesReq")
	}

	// then client context
	c.Assert(len(l.clientCtxs), Equals, 1)
	c.Assert(l.clientCtxs[Index(3)].clientId, Equals, Id(999))
}

func (t *T) TestLeaderShouldIncrementMatchIndexToLastIdxWhenReceiveSuccessRespFromFollower(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}}

	l.matchIndex[commCfg.cluster.Members[1]] = 0
	l.nextIndex[commCfg.cluster.Members[1]] = 1

	resp := Msg{
		Tp:   Rpc,
		From: commCfg.cluster.Members[1],
		To:   l.cfg.leader,
		Payload: &AppendEntriesResp{
			Term:    1,
			Success: true,
		},
	}

	// when
	for range l.log {
		_ = l.TakeAction(resp)
	}

	// then
	c.Assert(l.matchIndex[commCfg.cluster.Members[1]], Equals, Index(3))
	c.Assert(l.nextIndex[commCfg.cluster.Members[1]], Equals, Index(4))

	// when after that, matchIndex and nextIndex stayed
	_ = l.TakeAction(resp)

	// then
	c.Assert(l.matchIndex[commCfg.cluster.Members[1]], Equals, Index(3))
	c.Assert(l.nextIndex[commCfg.cluster.Members[1]], Equals, Index(4))
}

func (t *T) TestLeaderShouldIncrementCommittedIndexAndResponseToClientWhenReceiveMajoritySuccesses(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}}

	l.matchIndex[commCfg.cluster.Members[1]] = 1
	l.nextIndex[commCfg.cluster.Members[1]] = 2
	l.matchIndex[commCfg.cluster.Members[2]] = 1
	l.nextIndex[commCfg.cluster.Members[2]] = 2
	l.matchIndex[commCfg.cluster.Members[3]] = 0
	l.nextIndex[commCfg.cluster.Members[3]] = 1
	l.matchIndex[commCfg.cluster.Members[4]] = 1
	l.nextIndex[commCfg.cluster.Members[4]] = 2
	l.clientCtxs[Index(2)] = clientCtx{clientId: Id(999)}
	l.clientCtxs[Index(3)] = clientCtx{clientId: Id(999)}

	buildResp := func(id Id) Msg {
		return Msg{
			Tp:   Rpc,
			From: id,
			To:   l.cfg.leader,
			Payload: &AppendEntriesResp{
				Term:    1,
				Success: true,
			},
		}
	}

	// when
	res1 := l.TakeAction(buildResp(commCfg.cluster.Members[1]))
	res2 := l.TakeAction(buildResp(commCfg.cluster.Members[2]))
	_ = l.TakeAction(buildResp(commCfg.cluster.Members[1]))
	_ = l.TakeAction(buildResp(commCfg.cluster.Members[2]))

	// then
	c.Assert(l.matchIndex[commCfg.cluster.Members[1]], Equals, Index(3))
	c.Assert(l.nextIndex[commCfg.cluster.Members[1]], Equals, Index(4))
	c.Assert(l.matchIndex[commCfg.cluster.Members[2]], Equals, Index(3))
	c.Assert(l.nextIndex[commCfg.cluster.Members[2]], Equals, Index(4))
	c.Assert(l.matchIndex[commCfg.cluster.Members[3]], Equals, Index(0))
	c.Assert(l.nextIndex[commCfg.cluster.Members[3]], Equals, Index(1))
	c.Assert(l.matchIndex[commCfg.cluster.Members[4]], Equals, Index(1))
	c.Assert(l.nextIndex[commCfg.cluster.Members[4]], Equals, Index(2))

	c.Assert(res1, Equals, NullMsg)
	if msgs, ok := res2.Payload.([]Msg); ok {
		c.Assert(len(msgs), Equals, 1)

		for _, msg := range msgs {
			if payload, ok := msg.Payload.(*CmdResp); ok {
				c.Assert(payload.Success, Equals, true)
				c.Assert(msg.To, Equals, Id(999))

				_, exist := l.clientCtxs[Index(2)]
				c.Assert(exist, Equals, false)

				c.Assert(l.commitIndex, Equals, Index(3))
			} else {
				c.Fail()
				c.Logf("Should get CmdResp")
			}
		}
	} else {
		c.Fail()
		c.Logf("Should get Composed msg")
	}
}

func (t *T) TestLeaderWillBackToFollowerWhenReceiveAnyRpcWithNewTerm(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 3

	req := Msg{
		Tp: Rpc,
		Payload: &AppendEntriesReq{
			Term:         4,
			PrevLogTerm:  4,
			PrevLogIndex: 5,
			Entries:      []Entry{{Term: 4, Idx: 6, Cmd: ""}},
		},
	}

	// when
	res := l.TakeAction(req)

	// then
	c.Assert(res.Tp, Equals, MoveState)
	if f, ok := res.Payload.(*Follower); ok {
		c.Assert(f.currentTerm, Equals, (req.Payload.(*AppendEntriesReq)).Term)
	} else {
		c.Fail()
	}
}

func (t *T) TestLeaderShouldDecreaseNextIndexWhenReceiveFailureRespFromFollower(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}}

	fid := commCfg.cluster.Members[1]
	l.nextIndex[fid] = 4

	resp := Msg{
		Tp:   Rpc,
		From: fid,
		To:   l.cfg.leader,
		Payload: &AppendEntriesResp{
			Term:    1,
			Success: false,
		},
	}

	// when
	res := l.TakeAction(resp)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, fid)
	if req, ok := res.Payload.(*AppendEntriesReq); ok {
		c.Assert(req.Term, Equals, l.currentTerm)
		// nextIndex--
		c.Assert(l.nextIndex[fid], Equals, Index(3))
		c.Assert(req.PrevLogTerm, Equals, Term(1))
		c.Assert(req.PrevLogIndex, Equals, Index(1))
		// now the entries contain more than one
		c.Assert(req.Entries, DeepEquals, []Entry{{Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}})
	} else {
		c.Fail()
		c.Logf("Should get AppendEntriesReq")
	}
}

func (t *T) TestLeaderShouldKeepDecreaseNextIndexUntilFirstEntryWhenReceiveFailureRespFromFollower(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}}

	fid := commCfg.cluster.Members[1]
	l.nextIndex[fid] = 4

	resp := Msg{
		Tp:   Rpc,
		From: fid,
		To:   l.cfg.leader,
		Payload: &AppendEntriesResp{
			Term:    1,
			Success: false,
		},
	}

	// when
	_ = l.TakeAction(resp)
	_ = l.TakeAction(resp)
	res := l.TakeAction(resp)

	// then
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, fid)
	if req, ok := res.Payload.(*AppendEntriesReq); ok {
		c.Assert(req.Term, Equals, l.currentTerm)
		// nextIndex--
		c.Assert(l.nextIndex[fid], Equals, Index(1))
		c.Assert(req.PrevLogTerm, Equals, InvalidTerm)
		c.Assert(req.PrevLogIndex, Equals, InvalidIndex)
		// now the entries contain more than one
		c.Assert(req.Entries, DeepEquals, []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: "3"}})
	} else {
		c.Fail()
		c.Logf("Should get AppendEntriesReq")
	}
}

func (t *T) TestLeaderShouldNotCommitIfTheSatisfiedMajorityEntryIsNotAtCurrentTermUntilFirstCurrentTermEntryHasSuccessfullySatisfiedMajority(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 3
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 2, Idx: 2, Cmd: "2"}, {Term: 3, Idx: 3, Cmd: "3"}}

	fid0 := commCfg.cluster.Members[1]
	l.matchIndex[fid0] = 1
	l.nextIndex[fid0] = 2

	fid1 := commCfg.cluster.Members[2]
	l.matchIndex[fid1] = 3
	l.nextIndex[fid1] = 4

	// when
	_ = l.TakeAction(Msg{
		Tp:   Rpc,
		From: fid0,
		To:   l.cfg.leader,
		Payload: &AppendEntriesResp{
			Term:    3,
			Success: true,
		},
	})

	// then commitIndex not increase
	c.Assert(l.commitIndex, Equals, Index(1))
	c.Assert(l.matchIndex[fid0], Equals, Index(2))
	c.Assert(l.nextIndex[fid0], Equals, Index(3))

	// when
	_ = l.TakeAction(Msg{
		Tp:   Rpc,
		From: fid0,
		To:   l.cfg.leader,
		Payload: &AppendEntriesResp{
			Term:    3,
			Success: true,
		},
	})

	// then commitIndex will directly increase to term-3
	c.Assert(l.commitIndex, Equals, Index(3))
	c.Assert(l.matchIndex[fid0], Equals, Index(3))
	c.Assert(l.nextIndex[fid0], Equals, Index(4))
}

func (t *T) TestLeaderShouldReplaceConfigAndSaveOldConfigToLogWhenReceiveConfigChangeLog(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}}

	configChangeCmd := &ConfigChangeCmd{
		Members: []Id{190152, 96775, 2344359, 99811, 56867},
	}

	cmdReqMsg := Msg{
		Tp:   Cmd,
		From: Id(999),
		To:   l.cfg.leader,
		Payload: &CmdReq{
			Cmd: configChangeCmd,
		},
	}

	// when
	res := l.TakeAction(cmdReqMsg)

	// then msg
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, All)

	// then payload
	if appendLogReq, ok := res.Payload.(*AppendEntriesReq); ok {
		c.Assert(configChangeCmd.PrevMembers, DeepEquals, []Id{-11203, 190152, -2534, 96775, 2344359})
		c.Assert(appendLogReq.Entries, DeepEquals, []Entry{{Term: 1, Idx: 3, Cmd: configChangeCmd}})
	} else {
		c.Fail()
		c.Logf("Payload should be AppendEntriesReq")
	}

	// config changed
	c.Assert(l.cfg.cluster.Me, Equals, Id(-11203))
	c.Assert(l.cfg.cluster.Members, DeepEquals, []Id{190152, 96775, 2344359, 99811, 56867})
}

func (t *T) TestLeaderShouldRejectConfigChangeCmdWhenSomeUncommittedConfigChangeLogExist(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 1, Idx: 3, Cmd: &ConfigChangeCmd{
		Members: []Id{-11203, 190152, -2534, 96775, 2344359},
	}}}

	configChangeCmd := &ConfigChangeCmd{
		Members: []Id{-11203, 190152, -2534, 96775, 2344359, 56867},
	}

	cmdReqMsg := Msg{
		Tp:   Cmd,
		From: Id(999),
		To:   l.cfg.leader,
		Payload: &CmdReq{
			Cmd: configChangeCmd,
		},
	}

	// when
	res := l.TakeAction(cmdReqMsg)

	// then msg
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, cmdReqMsg.From)

	// then payload
	if cmdResp, ok := res.Payload.(*CmdResp); ok {
		c.Assert(cmdResp.Success, Equals, false)
	} else {
		c.Fail()
		c.Logf("Payload should be CmdResp")
	}

	// no change effected, no entry appended
	c.Assert(l.cfg.cluster.Me, Equals, Id(-11203))
	c.Assert(l.cfg.cluster.Members, DeepEquals, []Id{-11203, 190152, -2534, 96775, 2344359})
	c.Assert(l.getLastEntry().Idx, Equals, Index(3))
}

func (t *T) TestLeaderShouldFindServerThatAllLogReplicatedAndSendTimeoutNowReqToItWhenCallLeaderTransfer(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 2, Idx: 3, Cmd: "3"}}
	l.matchIndex[l.cfg.cluster.Members[1]] = 2
	l.matchIndex[l.cfg.cluster.Members[2]] = 1
	l.matchIndex[l.cfg.cluster.Members[3]] = 3
	l.matchIndex[l.cfg.cluster.Members[4]] = InvalidIndex

	// when
	res, ok := l.tryTransferLeadership()

	// then msg
	c.Assert(ok, Equals, true)
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, l.cfg.cluster.Members[3])

	// then payload
	if resp, ok := res.Payload.(*TimeoutNowReq); ok {
		c.Assert(resp.Term, Equals, l.currentTerm)
	} else {
		c.Fail()
		c.Logf("Payload should be TimeoutNowReq")
	}
}

func (t *T) TestLeaderShouldNotSendTimeoutNowReqIfNoFollowersCatchUpWithLog(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 2, Idx: 3, Cmd: "3"}}
	l.matchIndex[l.cfg.cluster.Members[1]] = 2
	l.matchIndex[l.cfg.cluster.Members[2]] = 1
	l.matchIndex[l.cfg.cluster.Members[3]] = 2
	l.matchIndex[l.cfg.cluster.Members[4]] = InvalidIndex

	// when
	res, ok := l.tryTransferLeadership()

	// then msg
	c.Assert(ok, Equals, false)
	c.Assert(res, Equals, NullMsg)
	c.Assert(l.inTransfer, Equals, false)
}

func (t *T) TestLeaderShouldRejectAnyCmdIfInTransferFlagIsTrue(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.inTransfer = true

	cmdReqMsg := Msg{
		Tp:   Cmd,
		From: Id(999),
		To:   l.cfg.leader,
		Payload: &CmdReq{
			Cmd: "",
		},
	}

	// when
	res := l.TakeAction(cmdReqMsg)

	// then msg
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, cmdReqMsg.From)

	if cmdResp, ok := res.Payload.(*CmdResp); ok {
		c.Assert(cmdResp.Success, Equals, false)
	} else {
		c.Fail()
		c.Logf("Payload should be CmdResp")
	}
}

func (t *T) TestLeaderShouldClearInTransferFlagAndGoBackToLeaderIfElectionTimeoutElapsedButNoHigherTermRpcReceived(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}}
	transferTo := commCfg.cluster.Members[1]
	l.matchIndex[transferTo] = 2

	// when election timeout
	l.inTransfer = true
	for i := int64(0); i < l.cfg.electionTimeout; i++ {
		_ = l.TakeAction(Msg{Tp: Tick})
	}

	// then msg
	c.Assert(l.inTransfer, Equals, false)
	c.Assert(l.cfg.tickCnt, Equals, int64(0))
}

func (t *T) TestLeaderShouldSetInTransferFlagWhenLatestLeaderEvictionConfigChangeCommitted(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 2
	l.commitIndex = 2
	l.lastApplied = 2
	l.cfg.cluster.Members = []Id{190152, -2534, 96775, 2344359}
	l.log = []Entry{
		{Term: 1, Idx: 1, Cmd: "1"},
		{Term: 1, Idx: 2, Cmd: "2"},
		{Term: 2, Idx: 3, Cmd: &ConfigChangeCmd{Members: []Id{190152, -2534, 96775, 2344359}}},
	}
	l.matchIndex[l.cfg.cluster.Members[1]] = 2
	l.nextIndex[l.cfg.cluster.Members[1]] = 3
	l.matchIndex[l.cfg.cluster.Members[2]] = 2
	l.nextIndex[l.cfg.cluster.Members[2]] = 3
	l.matchIndex[l.cfg.cluster.Members[3]] = 2
	l.nextIndex[l.cfg.cluster.Members[3]] = 3

	buildResp := func(id Id) Msg {
		return Msg{
			Tp:   Rpc,
			From: id,
			To:   l.cfg.leader,
			Payload: &AppendEntriesResp{
				Term:    2,
				Success: true,
			},
		}
	}

	// when the majority of members committed config change entry
	_ = l.TakeAction(buildResp(l.cfg.cluster.Members[1]))
	_ = l.TakeAction(buildResp(l.cfg.cluster.Members[2]))
	_ = l.TakeAction(buildResp(l.cfg.cluster.Members[3]))

	// then
	c.Assert(l.matchIndex[l.cfg.cluster.Members[1]], Equals, Index(3))
	c.Assert(l.matchIndex[l.cfg.cluster.Members[2]], Equals, Index(3))
	c.Assert(l.matchIndex[l.cfg.cluster.Members[3]], Equals, Index(3))

	// inTransfer should be set because of leader eviction
	c.Assert(l.inTransfer, Equals, true)
}

func (t *T) TestLeaderWillSendTimeoutNowReqToMostCatchUpFollowerWhenInTransferSetTrue(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 2, Idx: 3, Cmd: "3"}}
	l.matchIndex[l.cfg.cluster.Members[1]] = 2
	l.matchIndex[l.cfg.cluster.Members[2]] = 1
	l.matchIndex[l.cfg.cluster.Members[3]] = 3
	l.matchIndex[l.cfg.cluster.Members[4]] = InvalidIndex
	l.inTransfer = true

	// when
	res := l.TakeAction(Msg{Tp: Tick})

	// then msg
	c.Assert(res.Tp, Equals, Rpc)
	c.Assert(res.To, Equals, l.cfg.cluster.Members[3])

	// then payload
	if resp, ok := res.Payload.(*TimeoutNowReq); ok {
		c.Assert(resp.Term, Equals, l.currentTerm)
	} else {
		c.Fail()
		c.Logf("Payload should be TimeoutNowReq")
	}
}

func (t *T) TestLeaderWillNotSendTimeoutNowReqIfNoFollowerCatchUpWhenInTransferSetTrue(c *C) {
	// given
	l := NewFollower(commCfg, mockSm).toCandidate().toLeader()
	l.currentTerm = 1
	l.commitIndex = 1
	l.lastApplied = 1
	l.log = []Entry{{Term: 1, Idx: 1, Cmd: "1"}, {Term: 1, Idx: 2, Cmd: "2"}, {Term: 2, Idx: 3, Cmd: "3"}}
	l.matchIndex[l.cfg.cluster.Members[1]] = 2
	l.matchIndex[l.cfg.cluster.Members[2]] = 1
	l.matchIndex[l.cfg.cluster.Members[3]] = 2
	l.matchIndex[l.cfg.cluster.Members[4]] = InvalidIndex
	l.inTransfer = true

	// when
	res := l.TakeAction(Msg{Tp: Tick})

	// then payload
	if resp, ok := res.Payload.(*AppendEntriesReq); ok {
		c.Assert(len(resp.Entries), Equals, 0)
	} else {
		c.Fail()
		c.Logf("Payload should be AppendEntriesReq")
	}
}
