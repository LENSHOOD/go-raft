package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

func Execute() {
	root.AddCommand(server, client)
	bindServerFlags()
	bindClientFlags()

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

var root = &cobra.Command{
	Use:     "go-raft",
	Short:   "go-raft [OPTIONS] COMMAND",
	Long:    "go-raft is an implementation of Raft algorithm by golang",
	Version: "v0.1.0",
}

var server = &cobra.Command{
	Use:   "server",
	Short: "start raft server",
	Long:  "Start raft server init by given configs.",
	Run:   runServer,
}

var client = &cobra.Command{
	Use:   "client",
	Short: "start raft client",
	Long:  "Start raft client to send one command, waiting for response.",
	Run:   runClient,
}

type serverCmdFlags struct {
	me      string
	members []string
	tick    int64
	eleMax  int64
	eleMin  int64
}

var serverFlags = serverCmdFlags{
	me:     ":34220",
	tick:   10,
	eleMax: 300,
	eleMin: 100,
}

func bindServerFlags() {
	server.PersistentFlags().StringVarP(&serverFlags.me, "me", "", serverFlags.me, "self addr, [ip:port]")
	server.PersistentFlags().StringSliceVarP(&serverFlags.members, "members", "", serverFlags.members, "all cluster servers addr, [ip1:port1, ip2:port2, ...]")
	server.PersistentFlags().Int64VarP(&serverFlags.tick, "tick", "", serverFlags.tick, "tick interval as millisecond, [ms]")
	server.PersistentFlags().Int64VarP(&serverFlags.eleMax, "eleMax", "", serverFlags.eleMax, "max election timeout as n*tick, [n]")
	server.PersistentFlags().Int64VarP(&serverFlags.eleMin, "eleMin", "", serverFlags.eleMin, "min election timeout as n*tick, [n]")
}

type clientCmdFlags struct {
	leader string
	cmd    string
}

var clientFlags = clientCmdFlags{}

func bindClientFlags() {
	client.PersistentFlags().StringVarP(&clientFlags.leader, "leader", "", clientFlags.leader, "leader addr, [ip:port]")
	client.PersistentFlags().StringVarP(&clientFlags.cmd, "cmd", "", clientFlags.cmd, "command to be execute, [command content]")
}
