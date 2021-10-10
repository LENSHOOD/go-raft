package go_raft

import (
	"context"
	. "github.com/LENSHOOD/go-raft/api"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	"google.golang.org/grpc/peer"
)

type RaftServer struct {
	inputCh chan *mgr.Rpc
	mgr mgr.RaftManager
	UnimplementedRaftRpcServer
}

func (s *RaftServer) RequestVote(ctx context.Context, in *RequestVoteArguments) (*RequestVoteResults, error) {
	resp := s.serve(ctx, &core.RequestVoteReq{
		Term: core.Term(in.Term),
		CandidateId: core.Id(in.CandidateId),
		LastLogIndex: core.Index(in.LastLogIndex),
		LastLogTerm: core.Term(in.LastLogTerm),
	}).(*core.RequestVoteResp)

	return &RequestVoteResults{
		Term: int64(resp.Term),
		VoteGranted: resp.VoteGranted,
	}, nil
}
func (s *RaftServer) AppendEntries(ctx context.Context, in *AppendEntriesArguments) (*AppendEntriesResults, error) {
	resp := s.serve(ctx, &core.AppendEntriesReq{
		Term: core.Term(in.Term),
		LeaderId: core.Id(in.LeaderId),
		PrevLogIndex: core.Index(in.PrevLogIndex),
		PrevLogTerm: core.Term(in.PrevLogTerm),
		// Todo
		Entries: nil,
		LeaderCommit: core.Index(in.LeaderCommit),
	}).(*core.AppendEntriesResp)

	return &AppendEntriesResults{
		Term: int64(resp.Term),
		Success: resp.Success,
	}, nil
}
func (s *RaftServer) ExecCmd(ctx context.Context, in *CmdRequest) (*CmdResponse, error) {
	resp := s.serve(ctx, &core.CmdReq{
		Cmd: core.Command(in.Cmd),
	}).(*core.CmdResp)

	return &CmdResponse{
		Result: resp.Result.(string),
		Success: resp.Success,
	}, nil
}

func (s *RaftServer) serve(ctx context.Context, inPayload interface{}) (outPayload interface{}) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		panic("no peer")
	}

	addr := mgr.Address(p.Addr.String())
	outputCh := s.mgr.Dispatcher.RegisterResp(addr)
	s.inputCh <- &mgr.Rpc{
		Addr: addr,
		Payload: inPayload,
	}

	res := <-outputCh
	return res.Payload
}