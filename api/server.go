package api

import (
	"context"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"log"
	"time"
)

var logger = log.Default()

type RaftServer struct {
	inputCh chan *mgr.Rpc
	mgr     *mgr.RaftManager
	UnimplementedRaftRpcServer
}

func NewServer(inputCh chan *mgr.Rpc, mgr *mgr.RaftManager) *RaftServer {
	return &RaftServer{
		inputCh: inputCh,
		mgr:     mgr,
	}
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
		log.Fatalf("no peer")
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
	mgr      *mgr.RaftManager
	done     chan struct{}
}

func NewCaller(recv chan *mgr.Rpc, send chan *mgr.Rpc, mgr *mgr.RaftManager) (caller *Caller, done chan struct{}) {
	done = make(chan struct{})
	caller = &Caller{
		inputCh:  recv,
		outputCh: send,
		mgr:      mgr,
		done:     done,
	}

	return caller, done
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
	conn, err := grpc.Dial(
		string(rpc.Addr),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer(), otgrpc.LogPayloads())),
		grpc.WithStreamInterceptor(otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer(), otgrpc.LogPayloads())))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	ctx, canceled := context.WithDeadline(context.Background(), time.Now().Add(10*time.Millisecond))
	defer canceled()

	var resPayload interface{}
	switch rpc.Payload.(type) {
	case *core.RequestVoteReq:
		resp, err := NewRaftRpcClient(conn).RequestVote(ctx, MapToRequestVoteArguments(rpc.Payload.(*core.RequestVoteReq)))
		if err != nil {
			logger.Printf("[Caller] RequestVote error: %v", err)
		} else {
			resPayload = MapToRequestVoteResp(resp)
		}

	case *core.AppendEntriesReq:
		resp, err := NewRaftRpcClient(conn).AppendEntries(ctx, MapToAppendEntriesArguments(rpc.Payload.(*core.AppendEntriesReq)))
		if err != nil {
			logger.Printf("[Caller] AppendEntries error: %v", err)
		} else {
			resPayload = MapToAppendEntriesResp(resp)
		}
	}

	if resPayload != nil {
		c.outputCh <- &mgr.Rpc{
			Addr:    rpc.Addr,
			Payload: resPayload,
		}
	}
}
