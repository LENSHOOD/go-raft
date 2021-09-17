package core

type RequestVoteReq struct {
	Term Term
	CandidateId Id
	LastLogIndex Index
	LastLogTerm Term
}

type RequestVoteResp struct {
	Term Term
	VoteGranted bool
}

type AppendEntriesReq struct {
	Term Term
	LeaderId Id
	PrevLogTerm Term
	Entries []Entry
	LeaderCommit Index
}

type AppendEntriesResp struct {
	Term Term
	Success bool
}