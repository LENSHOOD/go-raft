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

type Rpc interface {
	GetTerm() Term
}

type RequestVoteReq struct {
	Term         Term
	CandidateId  Id
	LastLogIndex Index
	LastLogTerm  Term
}

func (rpc *RequestVoteReq) GetTerm() Term {
	return rpc.Term
}

type RequestVoteResp struct {
	Term        Term
	VoteGranted bool
}

func (rpc *RequestVoteResp) GetTerm() Term {
	return rpc.Term
}

type AppendEntriesReq struct {
	Term         Term
	LeaderId     Id
	PrevLogIndex Index
	PrevLogTerm  Term
	Entries      []Entry
	LeaderCommit Index
}

func (rpc *AppendEntriesReq) GetTerm() Term {
	return rpc.Term
}

type AppendEntriesResp struct {
	Term    Term
	Success bool
}

func (rpc *AppendEntriesResp) GetTerm() Term {
	return rpc.Term
}