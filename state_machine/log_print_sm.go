//go:build !chaos
// +build !chaos

package state_machine

import (
	"encoding/json"
	"github.com/LENSHOOD/go-raft/comm"
	"github.com/LENSHOOD/go-raft/core"
)

type LogPrintStateMachine struct{}

func (l *LogPrintStateMachine) Exec(cmd core.Command) interface{} {
	comm.GetLogger().Infof("Command Received: %s", cmd)
	if marshal, err := json.Marshal(cmd); err != nil {
		return "Received Cmd Marshal Error: " + err.Error()
	} else {
		return "Received: " + string(marshal)
	}
}

func BuildStateMachine() core.StateMachine {
	return &LogPrintStateMachine{}
}
