package subscription

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/gointerfaces/grpcutil"
	"github.com/ledgerwatch/erigon/core/types"
	types2 "github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/filters"
	"github.com/ledgerwatch/erigon/rlp"
	"github.com/ledgerwatch/erigon/turbo/rpchelper"
	proto_realtime "github.com/ledgerwatch/erigon/zk/realtime/subscription/proto"
	"github.com/ledgerwatch/log/v3"
	"google.golang.org/grpc"
)

type RealtimeFilters struct {
	txLock sync.RWMutex
	txSubs *rpchelper.SyncMap[SubID, Sub[*proto_realtime.RealtimeTransactionReply]]

	logsLock sync.RWMutex
	logsSubs *rpchelper.SyncMap[SubID, *LogsFilter]

	logger log.Logger
}

func NewRealtimeFilters(ctx context.Context, realtime proto_realtime.RealtimeClient, logger log.Logger) *RealtimeFilters {
	ff := &RealtimeFilters{
		txSubs:   rpchelper.NewSyncMap[SubID, Sub[*proto_realtime.RealtimeTransactionReply]](),
		logsSubs: rpchelper.NewSyncMap[SubID, *LogsFilter](),
		logger:   logger,
	}

	if realtime != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if err := ff.handleRealtimeTxMsgs(ctx, realtime); err != nil {
					select {
					case <-ctx.Done():
						return
					default:
					}
					if grpcutil.IsEndOfStream(err) || grpcutil.IsRetryLater(err) {
						time.Sleep(3 * time.Second)
						continue
					}
					logger.Warn("[Realtime rpc filters] error subscribing to realtime transaction", "err", err)
				}
			}
		}()

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if err := ff.handleRealtimeLogMsgs(ctx, realtime); err != nil {
					select {
					case <-ctx.Done():
						return
					default:
					}
					if grpcutil.IsEndOfStream(err) || grpcutil.IsRetryLater(err) {
						time.Sleep(3 * time.Second)
						continue
					}
					logger.Warn("[Realtime rpc filters] error subscribing to realtime logs", "err", err)
				}
			}
		}()
	}

	return ff
}

// ---------------------- Handles ----------------------
func (ff *RealtimeFilters) handleRealtimeTxMsgs(ctx context.Context, realtime proto_realtime.RealtimeClient) error {
	subscription, err := realtime.OnRealtimeTransaction(ctx, &proto_realtime.RealtimeTransactionRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		event, err := subscription.Recv()
		if errors.Is(err, io.EOF) {
			ff.logger.Debug("[Realtime rpc filters] the subscription to realtime transactions channel was closed")
			break
		}
		if err != nil {
			return err
		}

		ff.handleSendTxMsg(event)
	}
	return nil
}

func (ff *RealtimeFilters) handleSendTxMsg(reply *proto_realtime.RealtimeTransactionReply) {
	ff.txLock.RLock()
	defer ff.txLock.RUnlock()

	ff.txSubs.Range(func(k SubID, v Sub[*proto_realtime.RealtimeTransactionReply]) error {
		v.Send(reply)
		return nil
	})
}

func (ff *RealtimeFilters) handleRealtimeLogMsgs(ctx context.Context, realtime proto_realtime.RealtimeClient) error {
	subscription, err := realtime.OnRealtimeLogs(ctx, &proto_realtime.RealtimeLogsRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		event, err := subscription.Recv()
		if errors.Is(err, io.EOF) {
			ff.logger.Debug("[Realtime rpc filters] the subscription to realtime logs channel was closed")
			break
		}
		if err != nil {
			return err
		}

		ff.handleSendLogMsgs(event)
	}
	return nil
}

func (ff *RealtimeFilters) handleSendLogMsgs(reply *proto_realtime.RealtimeLogsReply) {
	ff.logsLock.RLock()
	defer ff.logsLock.RUnlock()

	if len(reply.RlpLogs) == 0 {
		return
	}

	logs := []*types.Log{}
	if err := rlp.Decode(bytes.NewReader(reply.RlpLogs), &logs); err != nil {
		ff.logger.Warn("OnRealtimeLogs rpc filters, unprocessable payload", "err", err)
	}

	for _, log := range logs {
		ff.logsSubs.Range(func(k SubID, filter *LogsFilter) error {
			if filter.allAddrs == 0 {
				_, addrOk := filter.addrs[log.Address]
				if !addrOk {
					return nil
				}
			}
			if filter.allTopics == 0 {
				if !chooseTopics(filter, log.Topics) {
					return nil
				}
			}
			lg := &types2.Log{
				Address:     log.Address,
				Topics:      log.Topics,
				Data:        log.Data,
				BlockNumber: log.BlockNumber,
				TxHash:      log.TxHash,
				TxIndex:     uint(log.TxIndex),
				BlockHash:   log.BlockHash,
				Index:       uint(log.Index),
				Removed:     log.Removed,
			}
			filter.sender.Send(lg)
			return nil
		})
	}
}

// ---------------------- Subscriptions ----------------------
func (ff *RealtimeFilters) SubscribeRealtimeTransactions(size int) (<-chan *proto_realtime.RealtimeTransactionReply, SubID) {
	ff.txLock.Lock()
	defer ff.txLock.Unlock()

	id := SubID(generateSubID())
	sub := newChanSub[*proto_realtime.RealtimeTransactionReply](size)
	ff.txSubs.Put(id, sub)
	return sub.ch, id
}

func (ff *RealtimeFilters) UnsubscribeRealtimeTransactions(id SubID) bool {
	ff.txLock.Lock()
	defer ff.txLock.Unlock()

	ch, ok := ff.txSubs.Get(id)
	if !ok {
		return false
	}
	ch.Close()
	if _, ok = ff.txSubs.Delete(id); !ok {
		return false
	}
	return true
}

func (ff *RealtimeFilters) SubscribeRealtimeLogs(size int, crit filters.FilterCriteria) (<-chan *types.Log, SubID) {
	ff.logsLock.Lock()
	defer ff.logsLock.Unlock()

	id := SubID(generateSubID())
	sub := newChanSub[*types.Log](size)
	filter := &LogsFilter{addrs: map[libcommon.Address]int{}, topics: map[libcommon.Hash]int{}, sender: sub}
	filter.addrs = map[libcommon.Address]int{}
	if len(crit.Addresses) == 0 {
		filter.allAddrs = 1
	} else {
		for _, addr := range crit.Addresses {
			filter.addrs[addr] = 1
		}
	}
	filter.topics = map[libcommon.Hash]int{}
	if len(crit.Topics) == 0 {
		filter.allTopics = 1
	} else {
		for _, topics := range crit.Topics {
			for _, topic := range topics {
				filter.topics[topic] = 1
			}
		}
	}
	filter.topicsOriginal = crit.Topics
	ff.logsSubs.Put(id, filter)

	return sub.ch, id
}

func (ff *RealtimeFilters) UnsubscribeRealtimeLogs(id SubID) bool {
	ff.logsLock.Lock()
	defer ff.logsLock.Unlock()

	filter, ok := ff.logsSubs.Get(id)
	if !ok {
		return false
	}
	filter.Close()
	if _, ok = ff.logsSubs.Delete(id); !ok {
		return false
	}
	return true
}
