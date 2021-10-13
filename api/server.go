package api

import (
	"context"
	"github.com/LENSHOOD/go-raft/core"
	"github.com/LENSHOOD/go-raft/mgr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"io"
	"log"
)

var logger = log.Default()

type RaftServer struct {
	inputCh chan *mgr.Rpc
	mgr     *mgr.RaftManager
	UnimplementedRaftRpcServer
}

func NewServer(inputCh chan *mgr.Rpc, mgr *mgr.RaftManager) *RaftServer {
	return &RaftServer{
		inputCh: inputCh,
		mgr:     mgr,
	}
}

func (s *RaftServer) RequestVote(stream RaftRpc_RequestVoteServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		resp := s.serve(stream.Context(), MapToRequestVoteReq(in)).(*core.RequestVoteResp)
		if err := stream.Send(MapToRequestVoteResults(resp)); err != nil {
			return err
		}
	}
}
func (s *RaftServer) AppendEntries(stream RaftRpc_AppendEntriesServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		resp := s.serve(stream.Context(), MapToAppendEntriesReq(in)).(*core.AppendEntriesResp)
		if err := stream.Send(MapToAppendEntriesResults(resp)); err != nil {
			return err
		}
	}
}
func (s *RaftServer) ExecCmd(ctx context.Context, in *CmdRequest) (*CmdResponse, error) {
	resp := s.serve(ctx, MapToCmdReq(in)).(*core.CmdResp)
	return MapToCmdResponse(resp), nil
}

func (s *RaftServer) serve(ctx context.Context, inPayload interface{}) (outPayload interface{}) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		log.Fatalf("no peer")
	}

	addr := mgr.Address(p.Addr.String())
	outputCh := s.mgr.Dispatcher.RegisterResp(addr)
	s.inputCh <- &mgr.Rpc{
		Addr:    addr,
		Payload: inPayload,
	}

	res := <-outputCh
	return res.Payload
}

type worker struct {
	inputCh  chan *mgr.Rpc
	outputCh chan *mgr.Rpc
	done     chan struct{}
	addr     mgr.Address
	conn     *grpc.ClientConn
	rv       RaftRpc_RequestVoteClient
	ae       RaftRpc_AppendEntriesClient
}

func (w *worker) run() {
	for {
		select {
		case <-w.done:
			_ = w.conn.Close()
			return
		case rpc := <-w.inputCh:
			w.sendReq(rpc)
		}
	}
}
func (w *worker) getConn() *grpc.ClientConn {
	if w.conn != nil {
		return w.conn
	}

	conn, err := grpc.Dial(string(w.addr), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	w.conn = conn
	return conn
}

func (w *worker) sendReq(rpc *mgr.Rpc) {
	var resPayload interface{}
	switch rpc.Payload.(type) {
	case *core.RequestVoteReq:
		if w.rv == nil {
			rv, err := NewRaftRpcClient(w.getConn()).RequestVote(context.Background())
			if err != nil {
				log.Fatalf("cannot get reuqest vote stream: %v", err)
			}
			w.rv = rv
		}

		if err := w.rv.Send(MapToRequestVoteArguments(rpc.Payload.(*core.RequestVoteReq))); err != nil {
			log.Fatalf("Failed to send: %v", err)
		}

		resp, err := w.rv.Recv()
		if err == io.EOF {
			_ = w.rv.CloseSend()
			w.rv = nil
			return
		}

		if err != nil {
			logger.Printf("[Caller] RequestVote error: %v", err)
		} else {
			resPayload = MapToRequestVoteResp(resp)
		}

	case *core.AppendEntriesReq:
		if w.ae == nil {
			ae, err := NewRaftRpcClient(w.getConn()).AppendEntries(context.Background())
			if err != nil {
				log.Fatalf("cannot get reuqest vote stream: %v", err)
			}
			w.ae = ae
		}

		if err := w.ae.Send(MapToAppendEntriesArguments(rpc.Payload.(*core.AppendEntriesReq))); err != nil {
			log.Fatalf("Failed to send: %v", err)
		}

		resp, err := w.ae.Recv()
		if err == io.EOF {
			_ = w.ae.CloseSend()
			w.ae = nil
			return
		}

		if err != nil {
			logger.Printf("[Caller] AppendEntries error: %v", err)
		} else {
			resPayload = MapToAppendEntriesResp(resp)
		}
	}

	if resPayload != nil {
		w.outputCh <- &mgr.Rpc{
			Addr:    rpc.Addr,
			Payload: resPayload,
		}
	}
}

type Caller struct {
	inputCh  chan *mgr.Rpc
	outputCh chan *mgr.Rpc
	mgr      *mgr.RaftManager
	done     chan struct{}
	workers  map[mgr.Address]*worker
}

func NewCaller(recv chan *mgr.Rpc, send chan *mgr.Rpc, manager *mgr.RaftManager) (caller *Caller, done chan struct{}) {
	done = make(chan struct{})
	caller = &Caller{
		inputCh:  recv,
		outputCh: send,
		mgr:      manager,
		done:     done,
		workers:  make(map[mgr.Address]*worker),
	}

	return caller, done
}

func (c *Caller) Run() {
	c.mgr.Dispatcher.RegisterReq(c.inputCh)

	for {
		select {
		case <-c.done:
			return
		case rpc := <-c.inputCh:
			w, exist := c.workers[rpc.Addr]
			if !exist {
				w = &worker{
					inputCh:  make(chan *mgr.Rpc, 10),
					outputCh: c.outputCh,
					done:     make(chan struct{}),
					addr:     rpc.Addr,
				}

				c.workers[rpc.Addr] = w
				go w.run()
			}

			w.inputCh <- rpc
		}
	}
}
