package jsonrpc

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common/debug"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/filters"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/zk/realtime/subscription"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
	"github.com/ledgerwatch/log/v3"
)

type RPCRealtimeTransaction struct {
	Tx       types.Transaction
	Receipt  *types.Receipt
	InnerTxs []*zktypes.InnerTx
}

// RealtimeTransactions send a notification each time when a transaction was received in real-time.
func (api *RealtimeAPIImpl) RealtimeTransactions(ctx context.Context, fullTx, includeExtraInfo *bool) (*rpc.Subscription, error) {
	if api.filters == nil {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		defer debug.LogPanic()
		protoCh, id := api.filters.SubscribeRealtimeTransactions(256)
		defer api.filters.UnsubscribeRealtimeTransactions(id)

		for {
			select {
			case proto, ok := <-protoCh:
				if proto != nil {
					tx, receipt, innerTxs, err := subscription.FromProtoTxMessage(proto)
					if err != nil {
						log.Warn("[realtime rpc] error while parsing transaction message from proto message", "err", err)
						return
					}
					if fullTx == nil || !*fullTx {
						err = notifier.Notify(rpcSub.ID, tx.Hash())
					} else {
						if includeExtraInfo == nil || !*includeExtraInfo {
							err = notifier.Notify(rpcSub.ID, tx)
						} else {
							err = notifier.Notify(rpcSub.ID, RPCRealtimeTransaction{
								Tx:       tx,
								Receipt:  receipt,
								InnerTxs: innerTxs,
							})
						}
					}

					if err != nil {
						log.Warn("[realtime rpc] error while notifying subscription", "err", err)
					}
				}
				if !ok {
					log.Warn("[realtime rpc] new realtime transactions channel was closed")
					return
				}
			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}

// Logs send a notification each time a new log appears in real-time.
func (api *RealtimeAPIImpl) Logs(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error) {
	if api.filters == nil {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		defer debug.LogPanic()
		logCh, id := api.filters.SubscribeRealtimeLogs(api.ethApi.SubscribeLogsChannelSize, crit)
		defer api.filters.UnsubscribeRealtimeLogs(id)

		for {
			select {
			case h, ok := <-logCh:
				if h != nil {
					if h.Topics == nil {
						h.Topics = make([]common.Hash, 0)
					}
					err := notifier.Notify(rpcSub.ID, h)
					if err != nil {
						log.Warn("[realtime rpc] error while notifying subscription", "err", err)
					}
				}
				if !ok {
					log.Warn("[realtime rpc] realtime log channel was closed")
					return
				}
			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}
