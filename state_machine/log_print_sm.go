package state_machine

import (
	"github.com/LENSHOOD/go-raft/core"
	"log"
)

var logger = log.Default()

type LogPrintStateMachine struct {}
func (l *LogPrintStateMachine) Exec(cmd core.Command) interface{} {
	logger.Printf("Command Received: %s", cmd)
	return nil
}
