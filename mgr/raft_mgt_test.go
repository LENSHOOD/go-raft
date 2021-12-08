package mgr

import (
	"context"
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
	Me:                   ":32104",
	Others:               []Address{"192.168.1.2:32104", "192.168.1.3:32104", "192.168.1.4:32104", "192.168.1.5:32104"},
	TickIntervalMilliSec: 30,
	ElectionTimeoutMin:   10,
	ElectionTimeoutMax:   50,
}

func (t *T) TestNewRaftMgr(c *C) {
	// when
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)

	// then
	_, isFollower := mgr.obj.(*core.Follower)
	c.Assert(isFollower, Equals, true)

	count := 0
	mgr.addrIdMapper.addrMapId.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	c.Assert(count, Equals, 5)

	count = 0
	mgr.addrIdMapper.idMapAddr.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	c.Assert(count, Equals, 5)
}

type fakeRaftObject struct{ mock.Mock }

func (f *fakeRaftObject) TakeAction(msg core.Msg) core.Msg {
	ret := f.Called(msg).Get(0).(core.Msg)
	if msg.Tp == core.Tick {
		return core.NullMsg
	}

	ret.From = msg.To
	if ret.To != core.All {
		ret.To = msg.From
	}
	return ret
}

func (f *fakeRaftObject) GetAllEntries() []core.Entry { return []core.Entry{} }

func (f *fakeRaftObject) GetCluster() core.Cluster {
	return f.Called().Get(0).(core.Cluster)
}

var standardCluster = core.Cluster{Me: -758425088686972977,
	Members: []core.Id{-758425088686972977, 1994190997193380571, 1295702547957371954, 6266824331869198845, -106856633615314508}}

func (t *T) TestTick(c *C) {
	// given
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.NullMsg)
	mgr.obj = mockObj

	// when
	go mgr.Run()
	time.Sleep(time.Millisecond * time.Duration(cfg.TickIntervalMilliSec*2))
	mgr.Stop()

	// then
	mockObj.AssertCalled(c, "TakeAction", core.Msg{Tp: core.Tick})
}

func (t *T) TestRaftMgrShouldChangeRaftObjWhenReceiveMoveStateMsg(c *C) {
	// given
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.Msg{
		Tp:      core.MoveState,
		Payload: &core.Follower{},
	})
	mockObj.On("GetCluster", mock.Anything).Return(standardCluster)
	mgr.obj = mockObj

	// when
	rpc := &Rpc{Ctx: context.TODO(), Addr: "", Payload: core.RequestVoteReq{Term: 10}}
	inputCh <- rpc
	go mgr.Run()
	for len(inputCh) != 0 {
	}
	mgr.Stop()

	// then
	mockObj.AssertExpectations(c)
	_, isFollower := mgr.obj.(*core.Follower)
	c.Assert(isFollower, Equals, true)
	id, isExist := mgr.addrIdMapper.addrMapId.Load(rpc.Addr)
	c.Assert(isExist, Equals, true)
	addr, _ := mgr.addrIdMapper.idMapAddr.Load(id)
	c.Assert(addr, Equals, rpc.Addr)
}

func (t *T) TestRaftMgrShouldRedirectMsgToRelateAddressWhenReceiveRpcMsg(c *C) {
	// given
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.Msg{
		Tp:      core.Rpc,
		Payload: &core.AppendEntriesResp{Term: 10},
	})
	mockObj.On("GetCluster", mock.Anything).Return(standardCluster)
	mgr.obj = mockObj

	// when
	addr := Address("addr")
	outputCh := mgr.Dispatcher.RegisterResp(addr)
	rpc := &Rpc{Ctx: context.TODO(), Addr: addr, Payload: core.AppendEntriesReq{Term: 10}}
	inputCh <- rpc
	go mgr.Run()
	for len(inputCh) != 0 {
	}

	// then
	res := <-outputCh
	mockObj.AssertExpectations(c)
	c.Assert(res.Payload.(*core.AppendEntriesResp).Term, Equals, core.Term(10))
	c.Assert(res.Addr, Equals, addr)
	mgr.Stop()
}

func (t *T) TestRaftMgrShouldRedirectMsgToAllOtherServerWhenReceiveRpcBroadcastMsg(c *C) {
	// given
	inputCh, outputCh := make(chan *Rpc, 10), make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)
	mgr.Dispatcher.RegisterReq(outputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.Msg{
		Tp:      core.Rpc,
		To:      core.All,
		Payload: &core.AppendEntriesReq{Term: 10},
	})
	mockObj.On("GetCluster", mock.Anything).Return(standardCluster)
	mgr.obj = mockObj

	// when
	addr := Address("addr")
	rpc := &Rpc{Ctx: context.TODO(), Addr: addr, Payload: core.AppendEntriesReq{Term: 10}}
	inputCh <- rpc
	go mgr.Run()
	for len(inputCh) != 0 || len(outputCh) == 0 {
	}
	mgr.Stop()

	// then
	mockObj.AssertExpectations(c)
	c.Assert(len(outputCh), Equals, len(mgr.cfg.Others))
	for i := 0; i < len(outputCh); i++ {
		res := <-outputCh
		id, exist := mgr.addrIdMapper.addrMapId.Load(res.Addr)
		c.Assert(exist, Equals, true)
		currAddr, _ := mgr.addrIdMapper.idMapAddr.Load(id)
		c.Assert(currAddr, Equals, res.Addr)
		c.Assert(res.Payload.(*core.AppendEntriesReq).Term, Equals, core.Term(10))
	}
}

func (t *T) TestRaftMgrShouldSetMsgTypeAsCmdWhenReceiveCmdMsg(c *C) {
	// given
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.NullMsg).Run(func(args mock.Arguments) {
		msg := args[0].(core.Msg)
		c.Assert(msg.Tp, Equals, core.Cmd)
	})
	mockObj.On("GetCluster", mock.Anything).Return(standardCluster)
	mgr.obj = mockObj

	// when
	cmd := &Rpc{Ctx: context.TODO(), Addr: "addr", Payload: &core.CmdReq{}}
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
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)
	mockObj := new(fakeRaftObject)
	mockObj.On("TakeAction", mock.Anything).Return(core.Msg{
		Tp:      core.Rpc,
		Payload: &core.CmdResp{Success: true},
	})
	mockObj.On("GetCluster", mock.Anything).Return(standardCluster)
	mgr.obj = mockObj

	// when
	addr := Address("addr")
	outputCh := mgr.Dispatcher.RegisterResp(addr)
	cmd := &Rpc{Ctx: context.TODO(), Addr: addr, Payload: &core.CmdReq{}}
	inputCh <- cmd
	go mgr.Run()
	for len(inputCh) != 0 {
	}

	// then
	res := <-outputCh
	mockObj.AssertExpectations(c)
	_, isCmdResp := res.Payload.(*core.CmdResp)
	c.Assert(isCmdResp, Equals, true)
	_, exist := mgr.addrIdMapper.addrMapId.Load(addr)
	c.Assert(exist, Equals, false)
	mgr.Stop()
}

func (t *T) TestDispatcherShouldDispatchRespToRelatedChannel(c *C) {
	// given
	d := dispatcher{}

	addr1 := Address("addr1")
	rpc1 := Rpc{Ctx: context.TODO(), Addr: addr1, Payload: &core.AppendEntriesResp{}}

	addr2 := Address("addr1")
	rpc2 := Rpc{Ctx: context.TODO(), Addr: addr2, Payload: &core.RequestVoteResp{}}

	addr3 := Address("addr1")
	rpc3 := Rpc{Ctx: context.TODO(), Addr: addr3, Payload: &core.CmdResp{}}

	// when
	assertDispatch := func(addr Address, rpc *Rpc) {
		ch := d.RegisterResp(addr)
		go d.dispatch(rpc)

		select {
		case v := <-ch:
			c.Assert(v, Equals, rpc)
		}

		d.Cancel(addr)
		_, exist := d.respOutputs.Load(addr)
		c.Assert(exist, Equals, false)
	}

	// then
	assertDispatch(addr1, &rpc1)
	assertDispatch(addr2, &rpc2)
	assertDispatch(addr3, &rpc3)
}

func (t *T) TestDispatcherShouldDispatchReqToRelatedChannel(c *C) {
	// given
	d := dispatcher{}
	reqCh := make(chan *Rpc, 3)

	addr := Address("addr")
	rpc1 := Rpc{Ctx: context.TODO(), Addr: addr, Payload: &core.AppendEntriesReq{}}
	rpc2 := Rpc{Ctx: context.TODO(), Addr: addr, Payload: &core.RequestVoteReq{}}

	// when
	d.RegisterReq(reqCh)
	d.dispatch(&rpc1)
	d.dispatch(&rpc2)

	// then
	arr := []*Rpc{&rpc1, &rpc2}
	for i := 0; i < 2; i++ {
		select {
		case v := <-reqCh:
			c.Assert(v, Equals, arr[i])
		}
	}

	close(reqCh)
}

func (t *T) TestDispatcherShouldRemoveAllChannelWhenCallClearAll(c *C) {
	// given
	d := dispatcher{}
	reqCh := make(chan *Rpc, 3)
	d.RegisterReq(reqCh)
	d.RegisterResp("1")
	d.RegisterResp("2")
	d.RegisterResp("3")

	// when
	d.clearAll()

	// then
	c.Assert(d.reqOutput, IsNil)
	_, exist := d.respOutputs.Load("1")
	c.Assert(exist, Equals, false)
	_, exist = d.respOutputs.Load("2")
	c.Assert(exist, Equals, false)
	_, exist = d.respOutputs.Load("3")
	c.Assert(exist, Equals, false)

	close(reqCh)
}

func (t *T) TestRaftMgrShouldConvertLeaderIdToLeaderAddressWhenReceiveFalseCmdResp(c *C) {
	// given
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)

	leaderAddr := mgr.cfg.Others[0]
	leaderId := mgr.getIdByAddr(leaderAddr)
	mockObj := new(fakeRaftObject)
	mockObj.
		On("TakeAction", mock.Anything).
		Return(core.Msg{
			Tp: core.Rpc,
			Payload: &core.CmdResp{
				Result:  leaderId,
				Success: false,
			}}).
		Run(func(args mock.Arguments) {
			msg := args[0].(core.Msg)
			if msg.Tp == core.Tick {
				return
			}

			c.Assert(msg.Tp, Equals, core.Cmd)
		})
	mockObj.On("GetCluster", mock.Anything).Return(standardCluster)
	mgr.obj = mockObj

	clientAddr := Address("addr")
	respCh := mgr.Dispatcher.RegisterResp(clientAddr)

	// when
	cmd := &Rpc{Ctx: context.TODO(), Addr: clientAddr, Payload: &core.CmdReq{}}
	inputCh <- cmd
	go mgr.Run()
	for len(inputCh) != 0 {
	}

	cmdRespMsg := <-respCh

	// then
	mockObj.AssertExpectations(c)
	if cmdResp, ok := cmdRespMsg.Payload.(*core.CmdResp); !ok {
		c.Fail()
	} else {
		c.Assert(cmdResp.Success, Equals, false)
		c.Assert(cmdResp.Result, Equals, leaderAddr)
	}

	mgr.Stop()
}

func (t *T) TestRaftMgrShouldReturnSelfAddressWhenReceiveFalseCmdRespWithInvalidLeaderId(c *C) {
	// given
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)

	mockObj := new(fakeRaftObject)
	mockObj.
		On("TakeAction", mock.Anything).
		Return(core.Msg{
			Tp: core.Rpc,
			Payload: &core.CmdResp{
				Result:  core.InvalidId,
				Success: false,
			}}).
		Run(func(args mock.Arguments) {
			msg := args[0].(core.Msg)
			if msg.Tp == core.Tick {
				return
			}

			c.Assert(msg.Tp, Equals, core.Cmd)
		})
	mockObj.On("GetCluster", mock.Anything).Return(standardCluster)
	mgr.obj = mockObj

	clientAddr := Address("addr")
	respCh := mgr.Dispatcher.RegisterResp(clientAddr)

	// when
	cmd := &Rpc{Ctx: context.TODO(), Addr: clientAddr, Payload: &core.CmdReq{}}
	inputCh <- cmd
	go mgr.Run()
	for len(inputCh) != 0 {
	}

	cmdRespMsg := <-respCh
	mgr.Stop()

	// then
	mockObj.AssertExpectations(c)
	if cmdResp, ok := cmdRespMsg.Payload.(*core.CmdResp); !ok {
		c.Fail()
	} else {
		c.Assert(cmdResp.Success, Equals, false)
		c.Assert(cmdResp.Result, Equals, mgr.cfg.Me)
	}
}

func (t *T) TestConfigChangeAddServerWillBeConvertToConfigChangeCmd(c *C) {
	// given
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)
	mockObj := new(fakeRaftObject)
	mockObj.
		On("GetCluster", mock.Anything).
		Return(core.Cluster{Me: 1, Members: []core.Id{1, 2, 3, 4, 5}})
	mgr.obj = mockObj

	serverToAdd := Address("127.0.0.1:10001")
	mgr.addrMapId.Store(serverToAdd, core.Id(6))

	// when
	cc := &ConfigChange{Op: Add, Server: serverToAdd}
	cmd := mgr.convertConfigChangeToCmd(cc)

	// then
	if resp, ok := cmd.(*core.ConfigChangeCmd); ok {
		c.Assert(resp.Members, DeepEquals, []core.Id{1, 2, 3, 4, 5, 6})
	} else {
		c.Fail()
		c.Logf("Payload should be ConfigChangeCmd")
	}
}

func (t *T) TestConfigChangeRemoveServerWillBeConvertToConfigChangeCmd(c *C) {
	// given
	inputCh := make(chan *Rpc, 10)
	mgr := NewRaftMgr(cfg, mockSm, inputCh)
	mockObj := new(fakeRaftObject)
	mockObj.
		On("GetCluster", mock.Anything).
		Return(core.Cluster{Me: 1, Members: []core.Id{1, 2, 3, 4, 5}})
	mgr.obj = mockObj

	serverToAdd := Address(":32104")
	mgr.addrMapId.Store(serverToAdd, core.Id(1))

	// when
	cc := &ConfigChange{Op: Remove, Server: serverToAdd}
	cmd := mgr.convertConfigChangeToCmd(cc)

	// then
	if resp, ok := cmd.(*core.ConfigChangeCmd); ok {
		c.Assert(resp.Members, DeepEquals, []core.Id{2, 3, 4, 5})
	} else {
		c.Fail()
		c.Logf("Payload should be ConfigChangeCmd")
	}
}
