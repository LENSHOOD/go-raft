package cmd

import (
	"context"
	"github.com/LENSHOOD/go-raft/api"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"log"
	"time"
)

func runClient(*cobra.Command, []string) {
	callAddr := clientFlags.leader

	resp := sendCmd(callAddr, clientFlags.cmd)
	for ; !resp.Success; resp = sendCmd(callAddr, clientFlags.cmd) {
		callAddr = resp.Result
	}

	log.Printf("Cmd executed, success: %t, result: %s", resp.Success, resp.Result)
}

func sendCmd(leaderAddr string, cmd string) *api.CmdResponse {
	conn, err := grpc.Dial(leaderAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	ctx, canceled := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer canceled()

	response, err := api.NewRaftRpcClient(conn).ExecCmd(ctx, &api.CmdRequest{Cmd: cmd})
	if err != nil {
		log.Fatalf("Error occured: %s", err.Error())
	}

	return response
}
