package state_machine

import (
	"encoding/json"
	"github.com/LENSHOOD/go-raft/core"
	"log"
)

var logger = log.Default()

type LogPrintStateMachine struct {}
func (l *LogPrintStateMachine) Exec(cmd core.Command) interface{} {
	logger.Printf("Command Received: %s", cmd)
	if marshal, err := json.Marshal(cmd); err != nil {
		return "Received Cmd Marshal Error: " + err.Error()
	} else {
		return "Received: " + string(marshal)
	}
}
