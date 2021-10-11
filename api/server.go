package go_raft

import (
	"context"
	"flag"
	. "github.com/LENSHOOD/go-raft/api"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	"github.com/LENSHOOD/go-raft/state_machine"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"log"
	"net"
	"strings"
)

type RaftServer struct {
	inputCh chan *mgr.Rpc
	mgr     *mgr.RaftManager
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
	conn, err := grpc.Dial(string(rpc.Addr), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	var resPayload interface{}
	switch rpc.Payload.(type) {
	case *core.RequestVoteReq:
		resp, err := NewRaftRpcClient(conn).RequestVote(context.Background(), MapToRequestVoteArguments(rpc.Payload.(*core.RequestVoteReq)))
		if err != nil {
			log.Fatalf("RequestVote error: %v", err)
		}

		resPayload = MapToRequestVoteResp(resp)

	case *core.AppendEntriesReq:
		resp, err := NewRaftRpcClient(conn).AppendEntries(context.Background(), MapToAppendEntriesArguments(rpc.Payload.(*core.AppendEntriesReq)))
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

func main() {
	// cfg
	me := flag.String("me", ":34220", "self addr, -me=[ip:port], default: 127.0.0.1:34220")
	othersStr := flag.String("others", "", "other cluster servers addr, -others=[ip1:port1, ip2:port2, ...]")
	var others []mgr.Address
	for i, v := range strings.FieldsFunc(*othersStr, func(r rune) bool { return r == ',' }) {
		others[i] = mgr.Address(strings.TrimSpace(v))
	}
	tick := flag.Int64("tick", 10, "tick interval as millisecond, -tick=[ms], default: 10ms")
	eleMax := flag.Int64("eleMax", 300, "max election timeout as millisecond, -eleMax=[ms], default: 300ms")
	eleMin := flag.Int64("eleMin", 100, "min election timeout as millisecond, -eleMin=[ms], default: 100ms")

	config := mgr.Config{
		Me:                   mgr.Address(*me),
		Others:               others,
		TickIntervalMilliSec: *tick,
		ElectionTimeoutMax:   *eleMax,
		ElectionTimeoutMin:   *eleMin,
	}

	// mgr
	inputCh := make(chan *mgr.Rpc, 10)
	raftMgr := mgr.NewRaftMgr(config, &state_machine.LogPrintStateMachine{}, inputCh)

	// caller
	clientRecvCh := make(chan *mgr.Rpc, 10)
	caller, _ := NewCaller(clientRecvCh, inputCh, raftMgr)
	go caller.Run()

	// server
	lis, err := net.Listen("tcp", string(config.Me))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	RegisterRaftRpcServer(s, &RaftServer{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}