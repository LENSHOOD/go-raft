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
	resp := s.serve(ctx, MapToRequestVoteReq(in)).(*core.RequestVoteResp)
	return MapToRequestVoteResults(resp), nil
}
func (s *RaftServer) AppendEntries(ctx context.Context, in *AppendEntriesArguments) (*AppendEntriesResults, error) {
	resp := s.serve(ctx, MapToAppendEntriesReq(in)).(*core.AppendEntriesResp)
	return MapToAppendEntriesResults(resp), nil
}
func (s *RaftServer) ExecCmd(ctx context.Context, in *CmdRequest) (*CmdResponse, error) {
	resp := s.serve(ctx, MapToCmdReq(in)).(*core.CmdResp)
	return MapToCmdResponse(resp), nil
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

	var resPayload interface{}
	switch rpc.Payload.(type) {
	case *core.RequestVoteReq:
		resp, err := c.client.RequestVote(context.Background(), MapToRequestVoteArguments(rpc.Payload.(*core.RequestVoteReq)))
		if err != nil {
			log.Fatalf("RequestVote error: %v", err)
		}

		resPayload = MapToRequestVoteResp(resp)

	case *core.AppendEntriesReq:
		resp, err := c.client.AppendEntries(context.Background(), MapToAppendEntriesArguments(rpc.Payload.(*core.AppendEntriesReq)))
		if err != nil {
			log.Fatalf("AppendEntries error: %v", err)
		}

		resPayload = MapToAppendEntriesResp(resp)
	}

	if resPayload != nil {
		c.outputCh <- &mgr.Rpc{
			Addr:    rpc.Addr,
			Payload: resPayload,
		}
	}
}