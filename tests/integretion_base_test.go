package tests

import (
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
	done chan struct{}
	svrs map[mgr.Address]*svr
	rw   sync.RWMutex
}

func newRouter() *router {
	return &router{done: make(chan struct{}), svrs: make(map[mgr.Address]*svr)}
}

func (r *router) register(svr *svr) {
	r.rw.Lock()
	defer r.rw.Unlock()
	r.svrs[svr.addr] = svr
}

func (r *router) run() {
	for _, svr := range r.svrs {
		go svr.mgr.Run()
	}

	send := func(addr mgr.Address, msg *mgr.Rpc) {
		r.rw.RLock()
		defer r.rw.RUnlock()
		target := msg.Addr
		msg.Addr = addr
		r.svrs[target].inputCh <- msg
	}

	for {
		for addr, svr := range r.svrs {
			select {
			case <-r.done:
				return
			case msg := <-svr.reqOutputCh:
				go send(addr, msg)
			default:
				for _, ch := range svr.respChs {
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
