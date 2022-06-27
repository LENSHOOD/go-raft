package cmd

import (
	"context"
	"github.com/LENSHOOD/go-raft/api"
	. "github.com/LENSHOOD/go-raft/comm"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"time"
)

func runClient(*cobra.Command, []string) {
	callAddr := clientFlags.leader

	resp := sendCmd(callAddr, clientFlags.cmd)
	for ; !resp.Success; resp = sendCmd(callAddr, clientFlags.cmd) {
		GetLogger().Infof("Redirect to: %s", resp.Result)
		callAddr = resp.Result
	}

	GetLogger().Infof("Cmd executed on %s, success: %t, result: %s", callAddr, resp.Success, resp.Result)
}

func sendCmd(leaderAddr string, cmd string) *api.CmdResponse {
	conn, err := grpc.Dial(leaderAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		GetLogger().Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	ctx, canceled := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer canceled()

	response, err := api.NewRaftRpcClient(conn).ExecCmd(ctx, &api.CmdRequest{Cmd: cmd})
	if err != nil {
		GetLogger().Fatalf("Error occured: %s", err.Error())
	}

	return response
}
