package tests

import (
	"context"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	. "gopkg.in/check.v1"
	"sync"
	"testing"
	"time"
)

// hook up go-check to go testing
func Test(t *testing.T) { TestingT(t) }

type T struct{}

var _ = Suite(&T{})

var srvAddrs = []mgr.Address{"192.168.1.1:32104", "192.168.1.2:32104", "192.168.1.3:32104", "192.168.1.4:32104", "192.168.1.5:32104"}
var commCfg = mgr.Config{
	TickIntervalMilliSec: 30,
	ElectionTimeoutMin:   10,
	ElectionTimeoutMax:   50,
	DebugMode:            true,
}

// mock state machine
type mockStateMachine struct{ cmds []core.Command }

func (m *mockStateMachine) Exec(cmd core.Command) interface{} {
	m.cmds = append(m.cmds, cmd)
	return cmd
}

type fakeTicker struct {
	ch chan time.Time
}

func (ft *fakeTicker) GetTickCh() <-chan time.Time { return ft.ch }
func (ft *fakeTicker) Start()                      {}
func (ft *fakeTicker) Stop()                       {}
func (ft *fakeTicker) tick()                       { ft.ch <- time.Now() }

type svr struct {
	addr        mgr.Address
	mgr         *mgr.RaftManager
	inputCh     chan *mgr.Rpc
	reqOutputCh chan *mgr.Rpc
	respChs     map[mgr.Address]<-chan *mgr.Rpc
	sm          *mockStateMachine
}

func newSvr(svrNo int, svrNum int) *svr {
	inputCh := make(chan *mgr.Rpc, 10)

	cfg := commCfg
	cfg.Me = srvAddrs[svrNo]
	for i, v := range srvAddrs {
		if i == svrNum {
			break
		}
		if i == svrNo {
			continue
		}
		cfg.Others = append(cfg.Others, v)
	}

	mockSm := &mockStateMachine{}
	//fTicker := &fakeTicker{make(chan time.Time)}
	//manager := mgr.NewRaftMgrWithTicker(cfg, mockSm, inputCh, fTicker)
	manager := mgr.NewRaftMgr(cfg, mockSm, inputCh)

	reqOutputCh := make(chan *mgr.Rpc, 10)
	manager.Dispatcher.RegisterReq(reqOutputCh)

	respChs := make(map[mgr.Address]<-chan *mgr.Rpc)
	for _, addr := range cfg.Others {
		ch := manager.Dispatcher.RegisterResp(addr)
		respChs[addr] = ch
	}

	return &svr{cfg.Me, manager, inputCh, reqOutputCh, respChs, mockSm}
}

type router struct {
	done         chan struct{}
	pauseCh      chan struct{}
	svrs         map[mgr.Address]*svr
	svrOutputChs map[mgr.Address][]<-chan *mgr.Rpc
	holds        sync.Map
	rw           sync.RWMutex
}

func newRouter() *router {
	return &router{done: make(chan struct{}), pauseCh: make(chan struct{}), svrs: make(map[mgr.Address]*svr), svrOutputChs: make(map[mgr.Address][]<-chan *mgr.Rpc)}
}

func (r *router) register(svr *svr) {
	r.rw.Lock()
	defer r.rw.Unlock()
	r.svrs[svr.addr] = svr
	for _, ch := range svr.respChs {
		r.svrOutputChs[svr.addr] = append(r.svrOutputChs[svr.addr], ch)
	}
	r.svrOutputChs[svr.addr] = append(r.svrOutputChs[svr.addr], svr.reqOutputCh)
}

func (r *router) add(svrToAdd *svr) {
	r.rw.Lock()
	defer r.rw.Unlock()
	for _, svr := range r.svrs {
		ch := svr.mgr.Dispatcher.RegisterResp(svrToAdd.addr)
		svr.respChs[svrToAdd.addr] = ch
		r.svrOutputChs[svr.addr] = append(r.svrOutputChs[svr.addr], ch)
	}

	for _, svr := range r.svrs {
		ch := svrToAdd.mgr.Dispatcher.RegisterResp(svr.addr)
		svrToAdd.respChs[svr.addr] = ch
		r.svrOutputChs[svrToAdd.addr] = append(r.svrOutputChs[svrToAdd.addr], ch)
	}
	r.svrOutputChs[svrToAdd.addr] = append(r.svrOutputChs[svrToAdd.addr], svrToAdd.reqOutputCh)
	r.svrs[svrToAdd.addr] = svrToAdd
}

func (r *router) run() {
	for _, svr := range r.svrs {
		go svr.mgr.Run()
	}

	send := func(sender mgr.Address, msg *mgr.Rpc) {
		// if sender or receiver already held, not send form/to it.
		receiver := msg.Addr
		_, senderExist := r.holds.Load(sender)
		_, receiverExist := r.holds.Load(receiver)
		if senderExist || receiverExist {
			return
		}

		r.rw.RLock()
		defer r.rw.RUnlock()
		msg.Addr = sender
		r.svrs[receiver].inputCh <- msg
	}

	for {
		select {
		case <-r.pauseCh:
			continue
		default:
		}

		for addr := range r.svrs {
			select {
			case <-r.done:
				return
			default:
				for _, ch := range r.svrOutputChs[addr] {
					select {
					case msg := <-ch:
						go send(addr, msg)
					default:
						continue
					}
				}
			}
		}
	}
}

func (r *router) pause() {
	close(r.pauseCh)
}

func (r *router) rerun() {
	r.pauseCh = make(chan struct{})
}

func (r *router) hold(svr *svr) {
	r.holds.Store(svr.addr, nil)
}

func (r *router) resume(svr *svr) {
	r.holds.Delete(svr.addr)
}

func (r *router) exec(svr *svr, cmd core.Command, clientAddr mgr.Address) {
	svr.inputCh <- &mgr.Rpc{
		Ctx:     context.TODO(),
		Addr:    clientAddr,
		Payload: &core.CmdReq{Cmd: cmd},
	}
}

func (r *router) changeSvr(svr *svr, cc *mgr.ConfigChange, clientAddr mgr.Address) {
	svr.inputCh <- &mgr.Rpc{
		Ctx:     context.TODO(),
		Addr:    clientAddr,
		Payload: cc,
	}
}

func waitCondition(condition func() bool, timeout time.Duration) (isTimeout bool) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	for !condition() {
		if d, ok := ctx.Deadline(); !ok || d.Before(time.Now()) {
			cancelFunc()
		}

		select {
		case <-ctx.Done():
			return true
		default:
		}
	}

	return false
}

func getLeaderWithException(svrs []*svr, exceptFor *svr) (leader *svr, found bool) {
	for _, svr := range svrs {
		if svr != exceptFor && svr.mgr.IsLeader() {
			return svr, true
		}
	}

	return nil, false
}

func getFollowers(svrs []*svr) []*svr {
	var followers []*svr
	for _, svr := range svrs {
		if svr.mgr.IsFollower() {
			followers = append(followers, svr)
		}
	}
	return followers
}

func waitCandidate(c *C, svrToBeCandidate *svr) {
	waitAllBecomeCandidate(c, []*svr{svrToBeCandidate})
}

func waitAllBecomeCandidate(c *C, svrs []*svr) {
	timeout := waitCondition(func() bool {
		for _, svr := range svrs {
			if !svr.mgr.IsCandidate() {
				return false
			}
		}
		return true
	}, 30*time.Second)
	if timeout {
		c.Errorf("No server turned to follower before time exceeded, test failed.")
		c.Fail()
	}
}

func waitLeader(c *C, svrs []*svr) *svr {
	return waitLeaderWithException(c, svrs, nil)
}

func waitLeaderWithException(c *C, svrs []*svr, exceptFor *svr) *svr {
	var leader *svr
	timeout := waitCondition(func() bool {
		l, ok := getLeaderWithException(svrs, exceptFor)
		leader = l
		return ok
	}, 30*time.Second)
	if timeout {
		c.Errorf("No server turned to leader before time exceeded, test failed.")
		c.FailNow()
	}

	return leader
}

func waitNumOfSvrLogLength(c *C, svrs []*svr, logLength int, numsOfSvrs int) {
	timeout := waitCondition(func() bool {
		cnt := 0
		for _, svr := range svrs {
			if len(svr.mgr.GetAllEntries()) >= logLength {
				cnt++
			}
		}

		return cnt >= numsOfSvrs
	}, 30*time.Second)
	if timeout {
		c.Errorf("Not enough (need &d) server's log length >= %d before time exceeded, test failed.", numsOfSvrs, logLength)
		c.FailNow()
	}
}
