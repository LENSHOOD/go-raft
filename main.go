package main

import (
	"flag"
	"github.com/LENSHOOD/go-raft/api"
	"github.com/LENSHOOD/go-raft/mgr"
	"github.com/LENSHOOD/go-raft/state_machine"
	"google.golang.org/grpc"
	"log"
	"net"
	"strings"
)

func main() {
	// cfg
	me := flag.String("me", ":34220", "self addr, -me=[ip:port], default: 127.0.0.1:34220")
	othersStr := flag.String("others", "", "other cluster servers addr, -others=[ip1:port1, ip2:port2, ...]")
	tick := flag.Int64("tick", 10, "tick interval as millisecond, -tick=[ms], default: 10ms")
	eleMax := flag.Int64("eleMax", 300, "max election timeout as n*tick, -eleMax=[n], default: 3000ms")
	eleMin := flag.Int64("eleMin", 100, "min election timeout as n*tick, -eleMin=[n], default: 1000ms")
	flag.Parse()

	var others []mgr.Address
	fieldsFunc := strings.FieldsFunc(*othersStr, func(r rune) bool { return r == ',' })
	for _, v := range fieldsFunc {
		others = append(others, mgr.Address(strings.TrimSpace(v)))
	}
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
	go raftMgr.Run()

	// caller
	clientRecvCh := make(chan *mgr.Rpc, 10)
	caller, _ := api.NewCaller(clientRecvCh, inputCh, raftMgr)
	go caller.Run()

	// server
	lis, err := net.Listen("tcp", string(config.Me))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	api.RegisterRaftRpcServer(s, api.NewServer(inputCh, raftMgr))
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
