package jsonrpc

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common/debug"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/filters"
	"github.com/ledgerwatch/erigon/rpc"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
	"github.com/ledgerwatch/log/v3"
)

const SubscribeTxMChannelSize = 256

type RPCRealtimeTransaction struct {
	Tx       types.Transaction
	Receipt  *types.Receipt
	InnerTxs []*zktypes.InnerTx
}

// RealtimeTransactions send a notification each time when a transaction was received in real-time.
func (api *RealtimeAPIImpl) RealtimeTransactions(ctx context.Context, fullTx, includeExtraInfo *bool) (*rpc.Subscription, error) {
	if !api.enableFlag || api.cacheDB == nil {
		return &rpc.Subscription{}, ErrRealtimeNotEnabled
	}

	if api.subService == nil {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()
	txChan, id, err := api.subService.SubscribeRealtimeTransactions(SubscribeTxMChannelSize)
	if err != nil {
		return &rpc.Subscription{}, err
	}

	go func() {
		defer debug.LogPanic()
		defer api.subService.UnsubscribeRealtimeTransactions(id)

		for {
			select {
			case txMsg, ok := <-txChan:
				if !ok {
					log.Warn("[realtime subscription] realtime txMsg channel closed")
					return
				}

				if err := txMsg.Validate(); err != nil {
					log.Warn("[realtime subscription] error while validating transaction message", "err", err)
					return
				}

				_, tx, receipt, innerTxs, err := txMsg.GetAllTxData()
				if err != nil {
					log.Warn("[realtime subscription] error getting tx data", "err", err)
					return
				}

				if fullTx == nil || !*fullTx {
					err = notifier.Notify(rpcSub.ID, tx.Hash())
					if err != nil {
						log.Warn("[realtime subscription] error while notifying subscription", "err", err)
					}
				} else {
					err = notifier.Notify(rpcSub.ID, RPCRealtimeTransaction{
						Tx:       tx,
						Receipt:  receipt,
						InnerTxs: innerTxs,
					})
				}
				if err != nil {
					log.Warn("[realtime subscription] error while notifying subscription", "err", err)
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
	if !api.enableFlag || api.cacheDB == nil {
		return &rpc.Subscription{}, ErrRealtimeNotEnabled
	}

	if api.subService == nil {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()
	logCh, id, err := api.subService.SubscribeRealtimeLogs(api.ethApi.SubscribeLogsChannelSize, crit)
	if err != nil {
		return &rpc.Subscription{}, err
	}

	go func() {
		defer debug.LogPanic()
		defer api.subService.UnsubscribeRealtimeLogs(id)

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
