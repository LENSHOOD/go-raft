package go_raft

import (
	"go-raft/core"
	. "gopkg.in/check.v1"
	"testing"
)

// hook up go-check to go testing
func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

// mock state machine
type mockStateMachine struct {}
func (m *mockStateMachine) Exec(cmd core.Command) interface{}  {
	return cmd
}
var mockSm = &mockStateMachine{}

func (t *T) TestNewRaftMgr(c *C) {
	// given
	cfg := Config{
		me: ":32104",
		others: []Address{"192.168.1.2:32104", "192.168.1.3:32104", "192.168.1.4:32104", "192.168.1.5:32104"},
		tickIntervalMilliSec: 30,
		electionTimeoutMin: 10,
		electionTimeoutMax: 50,
	}
	inputCh := make(chan *Rpc, 10)
	outputCh := make(chan *Rpc, 10)

	// when
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)

	// then
	_, isFollower := mgr.obj.(*core.Follower)
	c.Assert(isFollower, Equals, true)

	c.Assert(len(mgr.addrMapId), Equals, 5)
}