package go_raft

import (
	"context"
	. "github.com/LENSHOOD/go-raft/api"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"log"
)

type RaftServer struct {
	inputCh chan *mgr.Rpc
	mgr     mgr.RaftManager
	UnimplementedRaftRpcServer
}

func (s *RaftServer) RequestVote(ctx context.Context, in *RequestVoteArguments) (*RequestVoteResults, error) {
	resp := s.serve(ctx, &core.RequestVoteReq{
		Term:         core.Term(in.Term),
		CandidateId:  core.Id(in.CandidateId),
		LastLogIndex: core.Index(in.LastLogIndex),
		LastLogTerm:  core.Term(in.LastLogTerm),
	}).(*core.RequestVoteResp)

	return &RequestVoteResults{
		Term:        int64(resp.Term),
		VoteGranted: resp.VoteGranted,
	}, nil
}
func (s *RaftServer) AppendEntries(ctx context.Context, in *AppendEntriesArguments) (*AppendEntriesResults, error) {
	resp := s.serve(ctx, &core.AppendEntriesReq{
		Term:         core.Term(in.Term),
		LeaderId:     core.Id(in.LeaderId),
		PrevLogIndex: core.Index(in.PrevLogIndex),
		PrevLogTerm:  core.Term(in.PrevLogTerm),
		// Todo
		Entries:      nil,
		LeaderCommit: core.Index(in.LeaderCommit),
	}).(*core.AppendEntriesResp)

	return &AppendEntriesResults{
		Term:    int64(resp.Term),
		Success: resp.Success,
	}, nil
}
func (s *RaftServer) ExecCmd(ctx context.Context, in *CmdRequest) (*CmdResponse, error) {
	resp := s.serve(ctx, &core.CmdReq{
		Cmd: core.Command(in.Cmd),
	}).(*core.CmdResp)

	return &CmdResponse{
		Result:  resp.Result.(string),
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
		Addr:    addr,
		Payload: inPayload,
	}

	res := <-outputCh
	return res.Payload
}

type Caller struct {
	inputCh  chan *mgr.Rpc
	outputCh chan *mgr.Rpc
	mgr      mgr.RaftManager
	client   RaftRpcClient
	done     chan struct{}
}

func (c *Caller) Run() {
	c.mgr.Dispatcher.RegisterReq(c.inputCh)

	for {
		select {
		case <-c.done:
			return
		case rpc := <-c.inputCh:
			go c.sendReq(rpc)
		}
	}
}

func (c *Caller) sendReq(rpc *mgr.Rpc) {
	conn, err := grpc.Dial(string(rpc.Addr), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	var resultRpc *mgr.Rpc
	switch rpc.Payload.(type) {
	case *core.RequestVoteReq:
		req := rpc.Payload.(*core.RequestVoteReq)
		resp, err := c.client.RequestVote(context.Background(), &RequestVoteArguments{
			Term:         int64(req.Term),
			CandidateId:  int64(req.CandidateId),
			LastLogIndex: int64(req.LastLogIndex),
			LastLogTerm:  int64(req.LastLogTerm),
		})

		if err != nil {
			log.Fatalf("RequestVote error: %v", err)
		}

		resultRpc = &mgr.Rpc{
			Addr: rpc.Addr,
			Payload: &core.RequestVoteResp{
				Term:        core.Term(resp.Term),
				VoteGranted: resp.VoteGranted,
			},
		}
	case *core.AppendEntriesReq:
		req := rpc.Payload.(*core.AppendEntriesReq)
		resp, err := c.client.AppendEntries(context.Background(), &AppendEntriesArguments{
			Term:         int64(req.Term),
			LeaderId:     int64(req.LeaderId),
			PrevLogIndex: int64(req.PrevLogIndex),
			PrevLogTerm:  int64(req.PrevLogTerm),
			// TODO
			Entries:      nil,
			LeaderCommit: int64(req.LeaderCommit),
		})

		if err != nil {
			log.Fatalf("AppendEntries error: %v", err)
		}

		resultRpc = &mgr.Rpc{
			Addr: rpc.Addr,
			Payload: &core.AppendEntriesResp{
				Term:    core.Term(resp.Term),
				Success: resp.Success,
			},
		}
	}

	if resultRpc != nil {
		c.outputCh <- resultRpc
	}
}