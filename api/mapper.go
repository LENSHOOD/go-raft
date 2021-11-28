package api

import (
	"encoding/json"
	"github.com/LENSHOOD/go-raft/core"
)

func cmdMapToStr(cmd core.Command) string {
	marshal, err := json.Marshal(cmd)
	if err != nil {
		return err.Error()
	}

	return string(marshal)
}

func mapToRaftEntries(from []*Entry) (to []core.Entry) {
	if from == nil {
		return []core.Entry{}
	}

	for _, v := range from {
		to = append(to, core.Entry {
			Term: core.Term(v.Term),
			Idx: core.Index(v.Idx),
			Cmd: core.Command(v.Cmd),
		})
	}

	return to
}

func mapToApiEntries(from []core.Entry) (to []*Entry) {
	if len(from) == 0 {
		return nil
	}

	for _, v := range from {
		to = append(to, &Entry {
			Term: int64(v.Term),
			Idx: int64(v.Idx),
			Cmd: cmdMapToStr(v.Cmd),
		})
	}

	return to
}

func MapToAppendEntriesReq(from *AppendEntriesArguments) (to *core.AppendEntriesReq) {
	if from == nil {
		return nil
	}

	return &core.AppendEntriesReq{
		Term:         core.Term(from.Term),
		LeaderId:     core.Id(from.LeaderId),
		PrevLogIndex: core.Index(from.PrevLogIndex),
		PrevLogTerm:  core.Term(from.PrevLogTerm),
		Entries:      mapToRaftEntries(from.Entries),
		LeaderCommit: core.Index(from.LeaderCommit),
	}
}

func MapToAppendEntriesArguments(from *core.AppendEntriesReq) (to *AppendEntriesArguments) {
	if from == nil {
		return nil
	}

	return &AppendEntriesArguments{
		Term:         int64(from.Term),
		LeaderId:     int64(from.LeaderId),
		PrevLogIndex: int64(from.PrevLogIndex),
		PrevLogTerm:  int64(from.PrevLogTerm),
		Entries:      mapToApiEntries(from.Entries),
		LeaderCommit: int64(from.LeaderCommit),
	}
}

func MapToAppendEntriesResp(from *AppendEntriesResults) (to *core.AppendEntriesResp) {
	if from == nil {
		return nil
	}

	return &core.AppendEntriesResp{
		Term:    core.Term(from.Term),
		Success: from.Success,
	}
}

func MapToAppendEntriesResults(from *core.AppendEntriesResp) (to *AppendEntriesResults) {
	if from == nil {
		return nil
	}

	return &AppendEntriesResults{
		Term:        int64(from.Term),
		Success: from.Success,
	}
}

func MapToRequestVoteReq(from *RequestVoteArguments) (to *core.RequestVoteReq) {
	if from == nil {
		return nil
	}

	return &core.RequestVoteReq{
		Term:         core.Term(from.Term),
		CandidateId:  core.Id(from.CandidateId),
		LastLogIndex: core.Index(from.LastLogIndex),
		LastLogTerm:  core.Term(from.LastLogTerm),
	}
}

func MapToRequestVoteArguments(from *core.RequestVoteReq) (to *RequestVoteArguments) {
	if from == nil {
		return nil
	}

	return &RequestVoteArguments{
		Term:         int64(from.Term),
		CandidateId:  int64(from.CandidateId),
		LastLogIndex: int64(from.LastLogIndex),
		LastLogTerm:  int64(from.LastLogTerm),
	}
}

func MapToRequestVoteResp(from *RequestVoteResults) (to *core.RequestVoteResp) {
	if from == nil {
		return nil
	}

	return &core.RequestVoteResp{
		Term:        core.Term(from.Term),
		VoteGranted: from.VoteGranted,
	}
}

func MapToRequestVoteResults(from *core.RequestVoteResp) (to *RequestVoteResults) {
	if from == nil {
		return nil
	}

	return &RequestVoteResults{
		Term:        int64(from.Term),
		VoteGranted: from.VoteGranted,
	}
}

func MapToCmdReq(from *CmdRequest) (to *core.CmdReq) {
	if from == nil {
		return nil
	}

	return &core.CmdReq{
		Cmd: core.Command(from.Cmd),
	}
}

func MapToCmdRequest(from *core.CmdReq) (to *CmdRequest) {
	if from == nil {
		return nil
	}

	return &CmdRequest{
		Cmd: cmdMapToStr(from.Cmd),
	}
}

func MapToCmdResp(from *CmdResponse) (to *core.CmdResp) {
	if from == nil {
		return nil
	}

	return &core.CmdResp{
		Result:  from.Result,
		Success: from.Success,
	}
}

func MapToCmdResponse(from *core.CmdResp) (to *CmdResponse) {
	if from == nil {
		return nil
	}

	return &CmdResponse{
		Result:  from.Result.(string),
		Success: from.Success,
	}
}