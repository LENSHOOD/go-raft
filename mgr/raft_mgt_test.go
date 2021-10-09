package mgr

import (
	"github.com/LENSHOOD/go-raft/core"
	"github.com/stretchr/testify/mock"
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

func (t *T) TestNewRaftMgr(c *C) {
	// when
	inputCh, outputCh := make(chan *Rpc, 10), make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)

	// then
	_, isFollower := mgr.obj.(*core.Follower)
	c.Assert(isFollower, Equals, true)

	c.Assert(len(mgr.addrIdMapper.addrMapId), Equals, 5)
	c.Assert(len(mgr.addrIdMapper.idMapAddr), Equals, 5)
}

type fakeRaftObject struct{ mock.Mock }

func (f *fakeRaftObject) TakeAction(msg core.Msg) core.Msg {
	ret := f.Called(msg).Get(0).(core.Msg)
	ret.From = msg.To
	if ret.To != core.All {
		ret.To = msg.From
	}
	return ret
}

func (t *T) TestTick(c *C) {
	// given
	inputCh, outputCh := make(chan *Rpc, 10), make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.NullMsg)
	mgr.obj = mockObj

	// when
	go mgr.Run()
	time.Sleep(time.Millisecond * time.Duration(cfg.tickIntervalMilliSec*2))
	mgr.Stop()

	// then
	mockObj.AssertCalled(c, "TakeAction", core.Msg{Tp: core.Tick})
}

func (t *T) TestRaftMgrShouldChangeRaftObjWhenReceiveMoveStateMsg(c *C) {
	// given
	inputCh, outputCh := make(chan *Rpc, 10), make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.Msg{
		Tp:      core.MoveState,
		Payload: &core.Follower{},
	})
	mgr.obj = mockObj

	// when
	rpc := &Rpc{Addr: "", Payload: core.RequestVoteReq{Term: 10}}
	inputCh <- rpc
	go mgr.Run()
	for len(inputCh) != 0 {
	}
	mgr.Stop()

	// then
	mockObj.AssertExpectations(c)
	_, isFollower := mgr.obj.(*core.Follower)
	c.Assert(isFollower, Equals, true)
	id, isExist := mgr.addrIdMapper.addrMapId[rpc.Addr]
	c.Assert(isExist, Equals, true)
	addr, _ := mgr.addrIdMapper.idMapAddr[id]
	c.Assert(addr, Equals, rpc.Addr)
}

func (t *T) TestRaftMgrShouldRedirectMsgToRelateAddressWhenReceiveRpcMsg(c *C) {
	// given
	inputCh, outputCh := make(chan *Rpc, 10), make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.Msg{
		Tp:      core.Rpc,
		Payload: &core.AppendEntriesResp{Term: 10},
	})
	mgr.obj = mockObj

	// when
	addr := Address("addr")
	rpc := &Rpc{Addr: addr, Payload: core.AppendEntriesReq{Term: 10}}
	inputCh <- rpc
	go mgr.Run()
	for len(inputCh) != 0 {
	}
	mgr.Stop()

	// then
	mockObj.AssertExpectations(c)
	c.Assert(len(outputCh), Equals, 1)
	res := <-outputCh
	c.Assert(res.Payload.(*core.AppendEntriesResp).Term, Equals, core.Term(10))
	c.Assert(res.Addr, Equals, addr)
}

func (t *T) TestRaftMgrShouldRedirectMsgToAllOtherServerWhenReceiveRpcBroadcastMsg(c *C) {
	// given
	inputCh, outputCh := make(chan *Rpc, 10), make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.Msg{
		Tp:      core.Rpc,
		To:      core.All,
		Payload: &core.AppendEntriesResp{Term: 10},
	})
	mgr.obj = mockObj

	// when
	addr := Address("addr")
	rpc := &Rpc{Addr: addr, Payload: core.AppendEntriesReq{Term: 10}}
	inputCh <- rpc
	go mgr.Run()
	for len(inputCh) != 0 {
	}
	mgr.Stop()

	// then
	mockObj.AssertExpectations(c)
	c.Assert(len(outputCh), Equals, len(mgr.cfg.others))
	for i := 0; i < len(outputCh); i++ {
		res := <-outputCh
		id, exist := mgr.addrIdMapper.addrMapId[res.Addr]
		c.Assert(exist, Equals, true)
		c.Assert(mgr.addrIdMapper.idMapAddr[id], Equals, res.Addr)
		c.Assert(res.Payload.(*core.AppendEntriesResp).Term, Equals, core.Term(10))
	}
}

func (t *T) TestRaftMgrShouldSetMsgTypeAsCmdWhenReceiveCmdMsg(c *C) {
	// given
	inputCh, outputCh := make(chan *Rpc, 10), make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.NullMsg).Run(func(args mock.Arguments) {
		msg := args[0].(core.Msg)
		c.Assert(msg.Tp, Equals, core.Cmd)
	})
	mgr.obj = mockObj

	// when
	cmd := &Rpc{Addr: "addr", Payload: &core.CmdReq{}}
	inputCh <- cmd
	go mgr.Run()
	for len(inputCh) != 0 {
	}
	mgr.Stop()

	// then
	mockObj.AssertExpectations(c)
}

func (t *T) TestRaftMgrShouldRemoveClientIdAddrMappingWhenReceiveClientCmdRespMsg(c *C) {
	// given
	inputCh, outputCh := make(chan *Rpc, 10), make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh, outputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.Msg{
		Tp:      core.Rpc,
		Payload: &core.CmdResp{Success: true},
	})
	mgr.obj = mockObj

	// when
	addr := Address("addr")
	cmd := &Rpc{Addr: addr, Payload: &core.CmdReq{}}
	inputCh <- cmd
	go mgr.Run()
	for len(inputCh) != 0 {
	}
	mgr.Stop()

	// then
	mockObj.AssertExpectations(c)
	c.Assert(len(outputCh), Equals, 1)
	res := <-outputCh
	_, isCmdResp := res.Payload.(*core.CmdResp)
	c.Assert(isCmdResp, Equals, true)
	_, exist := mgr.addrIdMapper.addrMapId[addr]
	c.Assert(exist, Equals, false)
}