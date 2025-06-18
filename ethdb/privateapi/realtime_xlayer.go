package privateapi

import (
	"bytes"
	"context"
	"fmt"
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

	txReply, err := toRealtimeTransactionReply(msg, s.logger)
	if err != nil {
		return err
	}
	s.realtimeTxStreams.Broadcast(txReply, s.logger)

	var buf bytes.Buffer
	if err := rlp.Encode(&buf, msg.Receipt.Logs); err != nil {
		s.logger.Warn("failed to encode logs", "err", err)
		return err
	}

	logsReply := &proto_realtime.RealtimeLogsReply{RlpLogs: buf.Bytes()}
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

// ################ Helper Method ################

func toRealtimeTransactionReply(txMsg *kafkaTypes.TransactionMessage, logger log.Logger) (*proto_realtime.RealtimeTransactionReply, error) {
	if txMsg == nil {
		return nil, fmt.Errorf("transaction message is nil")
	}

	tx, _, err := txMsg.GetTransaction()
	if err != nil {
		logger.Warn("failed to get transaction from message", "err", err)
		return nil, err
	}

	var buf bytes.Buffer
	if err := tx.EncodeRLP(&buf); err != nil {
		logger.Warn("failed to encode transactions", "err", err)
		return nil, err
	}

	protoReply := &proto_realtime.RealtimeTransactionReply{
		RlpTransaction: buf.Bytes(), // No RLP transaction data in TransactionMessage
		Receipt:        &proto_realtime.Receipt{},
		InnerTxs:       make([]*proto_realtime.InnerTx, len(txMsg.InnerTxs)),
	}

	// Convert Receipt
	if txMsg.Receipt != nil {
		protoReply.Receipt = &proto_realtime.Receipt{
			Type:              uint32(txMsg.Receipt.Type),
			Root:              txMsg.Receipt.PostState,
			Status:            txMsg.Receipt.Status,
			CumulativeGasUsed: txMsg.Receipt.CumulativeGasUsed,
			LogsBloom:         txMsg.Receipt.Bloom[:],
			Logs:              make([]*proto_realtime.Log, len(txMsg.Receipt.Logs)),
			TransactionHash:   txMsg.Receipt.TxHash[:],
			ContractAddress:   txMsg.Receipt.ContractAddress[:],
			GasUsed:           txMsg.Receipt.GasUsed,
			BlockHash:         txMsg.Receipt.BlockHash[:],
			BlockNumber:       txMsg.Receipt.BlockNumber.String(),
			TransactionIndex:  uint64(txMsg.Receipt.TransactionIndex),
		}

		// Convert Logs
		for i, log := range txMsg.Receipt.Logs {
			protoTopics := make([][]byte, len(log.Topics))
			for j, topic := range log.Topics {
				protoTopics[j] = topic[:]
			}
			protoReply.Receipt.Logs[i] = &proto_realtime.Log{
				Address:          log.Address[:],
				Topics:           protoTopics,
				Data:             log.Data,
				BlockNumber:      log.BlockNumber,
				TransactionHash:  log.TxHash[:],
				TransactionIndex: uint64(log.TxIndex),
				BlockHash:        log.BlockHash[:],
				LogIndex:         uint64(log.Index),
				Removed:          log.Removed,
			}
		}
	} else {
		protoReply.Receipt = nil
	}

	// Convert InnerTxs
	for i, innerTx := range txMsg.InnerTxs {
		protoReply.InnerTxs[i] = &proto_realtime.InnerTx{
			Dept:          innerTx.Dept.String(),
			InternalIndex: innerTx.InternalIndex.String(),
			CallType:      innerTx.CallType,
			Name:          innerTx.Name,
			TraceAddress:  innerTx.TraceAddress,
			CodeAddress:   innerTx.CodeAddress,
			From:          innerTx.From,
			To:            innerTx.To,
			Input:         innerTx.Input,
			Output:        innerTx.Output,
			IsError:       innerTx.IsError,
			Gas:           innerTx.Gas,
			GasUsed:       innerTx.GasUsed,
			Value:         innerTx.Value,
			ValueWei:      innerTx.ValueWei,
			CallValueWei:  innerTx.CallValueWei,
			Error:         innerTx.Error,
		}
	}

	return protoReply, nil
}
