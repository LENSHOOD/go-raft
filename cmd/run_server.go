package cmd

import (
	"github.com/LENSHOOD/go-raft/api"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	"github.com/LENSHOOD/go-raft/state_machine"
	"github.com/LENSHOOD/go-raft/tracer"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"log"
	"net"
)

func runServer(*cobra.Command, []string) {
	var members []core.Address
	for _, v := range serverFlags.members {
		members = append(members, core.Address(v))
	}
	cls := core.Cluster{
		Me:      core.Address(serverFlags.me),
		Members: members,
	}

	config := mgr.Config{
		TickIntervalMilliSec: serverFlags.tick,
		ElectionTimeoutMax:   serverFlags.eleMax,
		ElectionTimeoutMin:   serverFlags.eleMin,
	}

	// tracer
	tracerCloser := tracer.InitGlobalTracer("raft-server: " + serverFlags.me)
	defer tracerCloser.Close()

	// mgr
	inputCh := make(chan *mgr.Rpc, 10)
	raftMgr := mgr.NewRaftMgr(cls, config, &state_machine.LogPrintStateMachine{}, inputCh)
	go raftMgr.Run()

	// caller
	clientRecvCh := make(chan *mgr.Rpc, 10)
	caller, _ := api.NewCaller(clientRecvCh, inputCh, raftMgr)
	go caller.Run()

	// server
	lis, err := net.Listen("tcp", string(cls.Me))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer(), otgrpc.LogPayloads())),
		grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(opentracing.GlobalTracer(), otgrpc.LogPayloads())),
	)
	api.RegisterRaftRpcServer(s, api.NewServer(inputCh, raftMgr))
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}