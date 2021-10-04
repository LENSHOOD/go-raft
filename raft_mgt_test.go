package go_raft

import (
	"go-raft/core"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

// hook up go-check to go testing
func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

// mock state machine
type mockStateMachine struct{}

func (m *mockStateMachine) Exec(cmd core.Command) interface{} {
	return cmd
}

var mockSm = &mockStateMachine{}

var cfg = Config{
	me:                   ":32104",
	others:               []Address{"192.168.1.2:32104", "192.168.1.3:32104", "192.168.1.4:32104", "192.168.1.5:32104"},
	tickIntervalMilliSec: 30,
	electionTimeoutMin:   10,
	electionTimeoutMax:   50,
}
var inputCh = make(chan *Rpc, 10)
var outputCh = make(chan *Rpc, 10)

func (t *T) TestNewRaftMgr(c *C) {
	// when
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)

	// then
	_, isFollower := mgr.obj.(*core.Follower)
	c.Assert(isFollower, Equals, true)

	c.Assert(len(mgr.addrMapId), Equals, 5)
}

type fakeRaftObject struct{ msg core.Msg}
func (f *fakeRaftObject) TakeAction(msg core.Msg) core.Msg {
	f.msg = msg
	return msg
}
var fakeRaftObj =  &fakeRaftObject{ msg: core.NullMsg}

func (t *T) TestTick(c *C) {
	// given
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)
	mgr.obj = fakeRaftObj

	// when
	mgr.Run()
	time.Sleep(time.Millisecond * time.Duration(cfg.tickIntervalMilliSec * 2))

	// then
	c.Assert(fakeRaftObj.msg.Tp, Equals, core.Tick)
}
