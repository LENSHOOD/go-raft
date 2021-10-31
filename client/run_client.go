package main

import (
	"context"
	"flag"
	"github.com/LENSHOOD/go-raft/api"
	"google.golang.org/grpc"
	"log"
	"time"
)

func main() {
	leaderAddr := flag.String("leader", "", "leader addr, -leader=[ip:port]")
	cmd := flag.String("cmd", "", "command to be execute, -cmd=[command content]")
	flag.Parse()

	callAddr := leaderAddr
	resp := &api.CmdResponse{Result: "nil-resp"}
	for resp = sendCmd(callAddr, cmd); !resp.Success; {
		callAddr = &resp.Result
	}

	log.Printf("Cmd executed, success: %t, result: %s", resp.Success, resp.Result)
}

func sendCmd(leaderAddr *string, cmd *string) *api.CmdResponse {
	conn, err := grpc.Dial(*leaderAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	ctx, canceled := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer canceled()

	response, err := api.NewRaftRpcClient(conn).ExecCmd(ctx, &api.CmdRequest{Cmd: *cmd})
	if err != nil {
		log.Fatalf("Error occured: %s", err.Error())
	}

	return response
}
