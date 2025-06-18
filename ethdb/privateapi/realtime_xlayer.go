package privateapi

import (
	"bytes"
	"context"
	"sync"

	proto_realtime "github.com/ledgerwatch/erigon-lib/gointerfaces/realtime"
	"github.com/ledgerwatch/erigon/rlp"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/realtime/kafka/types"
	"github.com/ledgerwatch/log/v3"
)

type RealtimeServer struct {
	proto_realtime.UnimplementedRealtimeServer
	ctx                 context.Context
	realtimeTxStreams   RealtimeTxStreams
	realtimeLogsStreams RealtimeLogsStreams
	logger              log.Logger
}

func NewRealtimeServer(ctx context.Context, logger log.Logger) *RealtimeServer {
	return &RealtimeServer{ctx: ctx, logger: logger}
}

func (s *RealtimeServer) OnRealtimeTransaction(req *proto_realtime.RealtimeTransactionRequest, reply proto_realtime.Realtime_OnRealtimeTransactionServer) error {
	remove := s.realtimeTxStreams.Add(reply)
	defer remove()
	select {
	case <-s.ctx.Done():
		return nil
	case <-reply.Context().Done():
		return nil
	}
}

func (s *RealtimeServer) OnRealtimeLogs(req *proto_realtime.RealtimeLogsRequest, reply proto_realtime.Realtime_OnRealtimeLogsServer) error {
	remove := s.realtimeLogsStreams.Add(reply)
	defer remove()
	select {
	case <-s.ctx.Done():
		return nil
	case <-reply.Context().Done():
		return nil
	}
}

func (s *RealtimeServer) BroadcastRealtimeTransactionMessage(msg *kafkaTypes.TransactionMessage) error {
	s.logger.Debug("BroadcastRealtimeTransactionMessage", "tx hash", msg.Hash, "block number", msg.BlockNumber, "tx index", msg.Receipt.TransactionIndex)
	tx, _, err := msg.GetTransaction()
	if err != nil {
		return err
	}

	var txBuf, logsBuf bytes.Buffer
	if err := tx.EncodeRLP(&txBuf); err != nil {
		s.logger.Warn("failed to encode transactions", "err", err)
		return err
	}
	if err := rlp.Encode(&logsBuf, msg.Receipt.Logs); err != nil {
		s.logger.Warn("failed to encode logs", "err", err)
		return err
	}

	txReply := &proto_realtime.RealtimeTransactionReply{RlpTransaction: txBuf.Bytes()}
	s.realtimeTxStreams.Broadcast(txReply, s.logger)

	logsReply := &proto_realtime.RealtimeLogsReply{RlpLogs: logsBuf.Bytes()}
	s.realtimeLogsStreams.Broadcast(logsReply, s.logger)

	return nil
}

// ################ Realtime Transaction Stream ################

type RealtimeTxStreams struct {
	chans  map[uint]proto_realtime.Realtime_OnRealtimeTransactionServer
	id     uint
	mu     sync.Mutex
	logger log.Logger
}

func (s *RealtimeTxStreams) Add(stream proto_realtime.Realtime_OnRealtimeTransactionServer) (remove func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.chans == nil {
		s.chans = make(map[uint]proto_realtime.Realtime_OnRealtimeTransactionServer)
	}
	s.id++
	id := s.id
	s.chans[id] = stream
	return func() { s.remove(id) }
}

func (s *RealtimeTxStreams) Broadcast(reply *proto_realtime.RealtimeTransactionReply, logger log.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, stream := range s.chans {
		err := stream.Send(reply)
		if err != nil {
			logger.Trace("failed send to realtime transaction stream", "err", err)
			select {
			case <-stream.Context().Done():
				delete(s.chans, id)
			default:
			}
		}
	}
}

func (s *RealtimeTxStreams) remove(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.chans[id]
	if !ok { // double-unsubscribe support
		return
	}
	delete(s.chans, id)
}

// ################ Realtime Logs Stream ################

type RealtimeLogsStreams struct {
	chans  map[uint]proto_realtime.Realtime_OnRealtimeLogsServer
	id     uint
	mu     sync.Mutex
	logger log.Logger
}

func (s *RealtimeLogsStreams) Add(stream proto_realtime.Realtime_OnRealtimeLogsServer) (remove func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.chans == nil {
		s.chans = make(map[uint]proto_realtime.Realtime_OnRealtimeLogsServer)
	}
	s.id++
	id := s.id
	s.chans[id] = stream
	return func() { s.remove(id) }
}

func (s *RealtimeLogsStreams) Broadcast(reply *proto_realtime.RealtimeLogsReply, logger log.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, stream := range s.chans {
		err := stream.Send(reply)
		if err != nil {
			logger.Trace("failed send to realtime logs stream", "err", err)
			select {
			case <-stream.Context().Done():
				delete(s.chans, id)
			default:
			}
		}
	}
}

func (s *RealtimeLogsStreams) remove(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.chans[id]
	if !ok { // double-unsubscribe support
		return
	}
	delete(s.chans, id)
}
