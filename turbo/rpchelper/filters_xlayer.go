package rpchelper

import (
	"bytes"
	"context"
	"errors"
	"io"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/gointerfaces"
	proto_realtime "github.com/ledgerwatch/erigon-lib/gointerfaces/realtime"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/filters"
	"github.com/ledgerwatch/erigon/rlp"
	"google.golang.org/grpc"
)

type (
	RealtimeTransactionSubID SubscriptionID
	RealtimeLogsSubID        SubscriptionID
)

// ################ Realtime Transacions ################

func (ff *Filters) SubscribeRealtimeTransactions(size int) (<-chan *types.Transaction, RealtimeTransactionSubID) {
	id := RealtimeTransactionSubID(generateSubscriptionID())
	sub := newChanSub[*types.Transaction](size)
	ff.realtimeTransactionSubs.Put(id, sub)
	return sub.ch, id
}

func (ff *Filters) UnsubscribeRealtimeTransactions(id RealtimeTransactionSubID) bool {
	ch, ok := ff.realtimeTransactionSubs.Get(id)
	if !ok {
		return false
	}
	ch.Close()
	if _, ok = ff.realtimeTransactionSubs.Delete(id); !ok {
		return false
	}
	return true
}

func (ff *Filters) subscribeToRealtimeTransactionMsgs(ctx context.Context, realtime proto_realtime.RealtimeClient) error {
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
			ff.logger.Debug("rpcdaemon: the subscription to realtime transactions channel was closed")
			break
		}
		if err != nil {
			return err
		}

		ff.HandleRealtmeTransaction(event)
	}
	return nil
}

func (ff *Filters) HandleRealtmeTransaction(reply *proto_realtime.RealtimeTransactionReply) {
	tx, err := types.DecodeTransaction(reply.RlpTransaction)
	if err != nil {
		ff.logger.Warn("OnRealtimeTransaction rpc filters, unprocessable tx payload", "err", err)
	}

	ff.mu.Lock()
	defer ff.mu.Unlock()

	ff.realtimeTransactionSubs.Range(func(k RealtimeTransactionSubID, v Sub[*types.Transaction]) error {
		v.Send(&tx)
		return nil
	})
}

// ################ Realtime Logs ################

func (ff *Filters) SubscribeRealtimeLogs(size int, crit filters.FilterCriteria) (<-chan *types.Log, RealtimeLogsSubID) {
	sub := newChanSub[*types.Log](size)
	id, f := ff.realtimeLogsSubs.insertRealtimeLogsFilter(sub)
	f.addrs = map[libcommon.Address]int{}
	if len(crit.Addresses) == 0 {
		f.allAddrs = 1
	} else {
		for _, addr := range crit.Addresses {
			f.addrs[addr] = 1
		}
	}
	f.topics = map[libcommon.Hash]int{}
	if len(crit.Topics) == 0 {
		f.allTopics = 1
	} else {
		for _, topics := range crit.Topics {
			for _, topic := range topics {
				f.topics[topic] = 1
			}
		}
	}
	f.topicsOriginal = crit.Topics
	ff.realtimeLogsSubs.addLogsFilters(f)
	// if any filter in the aggregate needs all addresses or all topics then the global log subscription needs to
	// allow all addresses or topics through
	lfr := ff.realtimeLogsSubs.createFilterRequest()
	addresses, topics := ff.realtimeLogsSubs.getAggMaps()
	for addr := range addresses {
		lfr.Addresses = append(lfr.Addresses, gointerfaces.ConvertAddressToH160(addr))
	}
	for topic := range topics {
		lfr.Topics = append(lfr.Topics, gointerfaces.ConvertHashToH256(topic))
	}

	return sub.ch, id
}

func (ff *Filters) UnsubscribeRealtimeLogs(id RealtimeLogsSubID) bool {
	isDeleted := ff.realtimeLogsSubs.removeRealtimeLogsFilter(id)
	// if any filters in the aggregate need all addresses or all topics then the request to the central
	// log subscription needs to honour this
	lfr := ff.realtimeLogsSubs.createFilterRequest()

	addresses, topics := ff.realtimeLogsSubs.getAggMaps()

	for addr := range addresses {
		lfr.Addresses = append(lfr.Addresses, gointerfaces.ConvertAddressToH160(addr))
	}
	for topic := range topics {
		lfr.Topics = append(lfr.Topics, gointerfaces.ConvertHashToH256(topic))
	}

	return isDeleted
}

func (ff *Filters) subscribeToRealtimeLogMsgs(ctx context.Context, realtime proto_realtime.RealtimeClient) error {
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
			ff.logger.Debug("rpcdaemon: the subscription to realtime logs channel was closed")
			break
		}
		if err != nil {
			return err
		}

		ff.HandleRealtmeLog(event)
	}
	return nil
}

func (ff *Filters) HandleRealtmeLog(reply *proto_realtime.RealtimeLogsReply) {
	if len(reply.RlpLogs) == 0 {
		return
	}
	l := []*types.Log{}
	if err := rlp.Decode(bytes.NewReader(reply.RlpLogs), &l); err != nil {
		ff.logger.Warn("OnRealtimeLogs rpc filters, unprocessable payload", "err", err)
	}

	ff.realtimeLogsSubs.distributeRealtimeLog(l)
}
