package subscription

import (
	"context"
	"io"

	"github.com/ledgerwatch/erigon/zk/realtime/subscription/proto"
	"google.golang.org/grpc"
)

type RealtimeClient struct {
	server proto.RealtimeServer
}

func NewRealtimeClient(server proto.RealtimeServer) *RealtimeClient {
	return &RealtimeClient{server: server}
}

func (s *RealtimeClient) OnRealtimeTransaction(ctx context.Context, in *proto.RealtimeTransactionRequest, opts ...grpc.CallOption) (proto.Realtime_OnRealtimeTransactionClient, error) {
	ch := make(chan *OnRealtimeTransactionReply, 16384)
	streamServer := &OnRealtimeTransactionStreamS{ch: ch, ctx: ctx}
	go func() {
		defer close(ch)
		streamServer.Err(s.server.OnRealtimeTransaction(in, streamServer))
	}()
	return &OnRealtimeTransactionStreamC{ch: ch, ctx: ctx}, nil
}

func (s *RealtimeClient) OnRealtimeLogs(ctx context.Context, in *proto.RealtimeLogsRequest, opts ...grpc.CallOption) (proto.Realtime_OnRealtimeLogsClient, error) {
	ch := make(chan *OnRealtimeLogsReply, 16384)
	streamServer := &OnRealtimeLogStreamS{ch: ch, ctx: ctx}
	go func() {
		defer close(ch)
		streamServer.Err(s.server.OnRealtimeLogs(in, streamServer))
	}()
	return &OnRealtimeLogStreamC{ch: ch, ctx: ctx}, nil
}

type OnRealtimeTransactionReply struct {
	r   *proto.RealtimeTransactionReply
	err error
}

type OnRealtimeTransactionStreamS struct {
	ch  chan *OnRealtimeTransactionReply
	ctx context.Context
	grpc.ServerStream
}

func (s *OnRealtimeTransactionStreamS) Send(m *proto.RealtimeTransactionReply) error {
	s.ch <- &OnRealtimeTransactionReply{r: m}
	return nil
}
func (s *OnRealtimeTransactionStreamS) Context() context.Context { return s.ctx }
func (s *OnRealtimeTransactionStreamS) Err(err error) {
	if err == nil {
		return
	}
	s.ch <- &OnRealtimeTransactionReply{err: err}
}

type OnRealtimeTransactionStreamC struct {
	ch  chan *OnRealtimeTransactionReply
	ctx context.Context
	grpc.ClientStream
}

func (c *OnRealtimeTransactionStreamC) Recv() (*proto.RealtimeTransactionReply, error) {
	m, ok := <-c.ch
	if !ok || m == nil {
		return nil, io.EOF
	}
	return m.r, m.err
}
func (c *OnRealtimeTransactionStreamC) Context() context.Context { return c.ctx }

// ################ Realtime Logs ################

type OnRealtimeLogsReply struct {
	r   *proto.RealtimeLogsReply
	err error
}

type OnRealtimeLogStreamS struct {
	ch  chan *OnRealtimeLogsReply
	ctx context.Context
	grpc.ServerStream
}

func (s *OnRealtimeLogStreamS) Send(m *proto.RealtimeLogsReply) error {
	s.ch <- &OnRealtimeLogsReply{r: m}
	return nil
}
func (s *OnRealtimeLogStreamS) Context() context.Context { return s.ctx }
func (s *OnRealtimeLogStreamS) Err(err error) {
	if err == nil {
		return
	}
	s.ch <- &OnRealtimeLogsReply{err: err}
}

type OnRealtimeLogStreamC struct {
	ch  chan *OnRealtimeLogsReply
	ctx context.Context
	grpc.ClientStream
}

func (c *OnRealtimeLogStreamC) Recv() (*proto.RealtimeLogsReply, error) {
	m, ok := <-c.ch
	if !ok || m == nil {
		return nil, io.EOF
	}
	return m.r, m.err
}
func (c *OnRealtimeLogStreamC) Context() context.Context { return c.ctx }
