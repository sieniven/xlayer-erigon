package jsonrpc

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/common"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
)

func (api *RealtimeAPIImpl) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	txn, receipt, _, ok := api.txInfoMap.Get(hash)
	if !ok {
		return api.ethApi.GetTransactionReceipt(ctx, hash)
	}
	header, ok := api.headerMap.Get(receipt.BlockNumber.Uint64())
	if !ok {
		return api.ethApi.GetTransactionReceipt(ctx, hash)
	}

	tx, err := api.ethApi.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	cc, err := api.ethApi.chainConfig(ctx, tx)
	if err != nil {
		return nil, err
	}

	return marshalReceipt(receipt, txn, cc, header, txn.Hash(), true), nil
}

func (api *RealtimeAPIImpl) GetInternalTransactions(ctx context.Context, hash common.Hash) ([]*zktypes.InnerTx, error) {
	_, _, innerTxs, ok := api.txInfoMap.Get(hash)
	if !ok {
		return api.ethApi.GetInternalTransactions(ctx, hash)
	}

	return innerTxs, nil
}
