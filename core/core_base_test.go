package core

import (
	. "gopkg.in/check.v1"
	"testing"
)

// hook up go-check to go testing
func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

var commCfg = Config{
	cluster: Cluster{
		Me:      "192.168.1.1:32104",
		Members: []Address{"192.168.1.1:32104", "192.168.1.2:32104", "192.168.1.3:32104", "192.168.1.4:32104", "192.168.1.5:32104"},
	},
	leader:             InvalidId,
	electionTimeoutMin: 3,
	electionTimeoutMax: 10,
	electionTimeout:    3,
	tickCnt:            0,
}

type mockStateMachine struct{}

func (m *mockStateMachine) Exec(cmd Command) interface{} {
	return cmd
}

var mockSm = &mockStateMachine{}
