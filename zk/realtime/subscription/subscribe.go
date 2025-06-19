package subscription

import (
	"context"
	"sync"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/filters"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/realtime/kafka/types"
	"github.com/ledgerwatch/log/v3"
)

// Buffer size should be large enough, and limit the number of subscriptions on the node.
const DEFAULT_BUFFER_SIZE = 1000

type RealtimeSubscription struct {
	txSubs    *SyncMap[SubID, Sub[*kafkaTypes.TransactionMessage]]
	logsSubs  *SyncMap[SubID, *LogsFilter]
	newTxChan chan *kafkaTypes.TransactionMessage
	logger    log.Logger
}

func NewRealtimeSubscription(ctx context.Context, logger log.Logger) *RealtimeSubscription {
	return &RealtimeSubscription{
		txSubs:    NewSyncMap[SubID, Sub[*kafkaTypes.TransactionMessage]](),
		logsSubs:  NewSyncMap[SubID, *LogsFilter](),
		newTxChan: make(chan *kafkaTypes.TransactionMessage, DEFAULT_BUFFER_SIZE),
		logger:    logger,
	}
}

func (ff *RealtimeSubscription) Start(ctx context.Context) {
	var wg sync.WaitGroup
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case txMsg := <-ff.newTxChan:
				wg.Add(2)
				go func() {
					defer wg.Done()
					ff.handleRealtimeTxMsgs(ctx, txMsg)
				}()
				go func() {
					defer wg.Done()
					ff.handleRealtimeLogMsgs(ctx, txMsg)
				}()
				wg.Wait()
			default:
			}
		}
	}()
}

func (ff *RealtimeSubscription) handleRealtimeTxMsgs(ctx context.Context, txMsg *kafkaTypes.TransactionMessage) {
	ff.txSubs.Range(func(k SubID, v Sub[*kafkaTypes.TransactionMessage]) error {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		v.Send(txMsg)
		return nil
	})
}

func (ff *RealtimeSubscription) handleRealtimeLogMsgs(ctx context.Context, txMsg *kafkaTypes.TransactionMessage) {
	logs := txMsg.Receipt.Logs
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
			filter.sender.Send(log)
			return nil
		})
	}
}

func (ff *RealtimeSubscription) BroadcastNewTxMsg(txMsg *kafkaTypes.TransactionMessage) {
	ff.newTxChan <- txMsg
}

func (ff *RealtimeSubscription) SubscribeRealtimeTransactions(size int) (<-chan *kafkaTypes.TransactionMessage, SubID) {
	id := SubID(generateSubID())
	sub := newChanSub[*kafkaTypes.TransactionMessage](size)
	ff.txSubs.Put(id, sub)
	return sub.ch, id
}

func (ff *RealtimeSubscription) UnsubscribeRealtimeTransactions(id SubID) bool {
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

func (ff *RealtimeSubscription) SubscribeRealtimeLogs(size int, crit filters.FilterCriteria) (<-chan *types.Log, SubID) {
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

func (ff *RealtimeSubscription) UnsubscribeRealtimeLogs(id SubID) bool {
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
