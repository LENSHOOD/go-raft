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
		Me:     1,
		Others: []Id{2, 3, 4, 5},
	},
	electionTimeoutMin: 3,
	electionTimeoutMax: 10,
	electionTimeout:    3,
	tickCnt:            0,
}

type mockStateMachine struct {}
func (m *mockStateMachine) exec(cmd Command) interface{}  {
	return cmd
}
var mockSm = &mockStateMachine{}
