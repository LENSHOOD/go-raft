package core

type MsgType int

const (
	Tick MsgType = iota
	MoveState
	Rpc
	Cmd
	Null
)

type Msg struct {
	tp      MsgType
	from    Id
	to      Id
	payload interface{}
}

var NullMsg = Msg{tp: Null}

type TermHolder interface {
	GetTerm() Term
}

type RequestVoteReq struct {
	Term         Term
	CandidateId  Id
	LastLogIndex Index
	LastLogTerm  Term
}

func (th *RequestVoteReq) GetTerm() Term {
	return th.Term
}

type RequestVoteResp struct {
	Term        Term
	VoteGranted bool
}

func (th *RequestVoteResp) GetTerm() Term {
	return th.Term
}

type AppendEntriesReq struct {
	Term         Term
	LeaderId     Id
	PrevLogIndex Index
	PrevLogTerm  Term
	Entries      []Entry
	LeaderCommit Index
}

func (th *AppendEntriesReq) GetTerm() Term {
	return th.Term
}

type AppendEntriesResp struct {
	Term    Term
	Success bool
}

func (th *AppendEntriesResp) GetTerm() Term {
	return th.Term
}

type CmdReq struct {
	Cmd Command
}

type CmdResp struct {
	Success bool
}