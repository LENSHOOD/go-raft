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
	Tp      MsgType
	From    Address
	To      Address
	Payload interface{}
}

var NullMsg = Msg{Tp: Null}

type TermHolder interface {
	GetTerm() Term
}

type RequestVoteReq struct {
	Term           Term
	CandidateId    Address
	LastLogIndex   Index
	LastLogTerm    Term
	LeaderTransfer bool
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
	LeaderId     Address
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
	Result  interface{}
	Success bool
}

// ConfigChangeCmd save the current and previous config snapshot
//     The main reason of not use rpc like {Op: Add/Remove, Address: "some-addr"}
// is that:
//
//     ConfigChangeCmd act as an Entry to replicate self all across the cluster,
// we can hardly to maintain this special Entry will be applied only once (unlike
// other Entries, ConfigChangeCmd must apply before commit). Obviously stateless
// cmd is more robust than stateful. (just like declarative vs. imperative)
type ConfigChangeCmd struct {
	Members     []Address
	PrevMembers []Address
}

type TimeoutNowReq struct {
	Term Term
}

func (th *TimeoutNowReq) GetTerm() Term {
	return th.Term
}
