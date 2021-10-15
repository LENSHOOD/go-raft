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

var srvAddrs = []mgr.Address{"192.168.1.1:32104", "192.168.1.2:32104", "192.168.1.3:32104"}
var commCfg = mgr.Config{
	TickIntervalMilliSec: 30,
	ElectionTimeoutMin:   10,
	ElectionTimeoutMax:   50,
	DebugMode:            true,
}

// mock state machine
type mockStateMachine struct{}

func (m *mockStateMachine) Exec(cmd core.Command) interface{} { return cmd }

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
}

func newSvr(srvNo int) *svr {
	inputCh := make(chan *mgr.Rpc, 10)

	cfg := commCfg
	cfg.Me = srvAddrs[srvNo]
	for i, v := range srvAddrs {
		if i == srvNo {
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

	return &svr{cfg.Me, manager, inputCh, reqOutputCh, respChs}
}

type router struct {
	done         chan struct{}
	svrs         map[mgr.Address]*svr
	svrOutputChs map[mgr.Address][]<-chan *mgr.Rpc
	holds        sync.Map
	rw           sync.RWMutex
}

func newRouter() *router {
	return &router{done: make(chan struct{}), svrs: make(map[mgr.Address]*svr), svrOutputChs: make(map[mgr.Address][]<-chan *mgr.Rpc)}
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

func (r *router) hold(svr *svr) {
	r.holds.Store(svr.addr, nil)
}

func (r *router) resume(svr *svr) {
	r.holds.Delete(svr.addr)
}

func (r *router) exec(svr *svr, cmd core.Command, clientAddr mgr.Address) {
	svr.inputCh <- &mgr.Rpc{
		Addr:    clientAddr,
		Payload: &core.CmdReq{Cmd: cmd},
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
