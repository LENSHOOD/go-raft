package core

type MsgType int

const (
	Tick MsgType = iota
	MoveState
	Req
	Resp
	Null
)

type Msg struct {
	tp      MsgType
	from    Id
	to      Id
	payload interface{}
}

var NullMsg = Msg{tp: Null}

type TickOrReq struct {
	Req interface{}
}

type RequestVoteReq struct {
	Term         Term
	CandidateId  Id
	LastLogIndex Index
	LastLogTerm  Term
}

type RequestVoteResp struct {
	Term        Term
	VoteGranted bool
}

type AppendEntriesReq struct {
	Term         Term
	LeaderId     Id
	PrevLogIndex Index
	PrevLogTerm  Term
	Entries      []Entry
	LeaderCommit Index
}

type AppendEntriesResp struct {
	Term    Term
	Success bool
}
