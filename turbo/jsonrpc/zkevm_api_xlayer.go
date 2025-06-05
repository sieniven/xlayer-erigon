package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/rpc"
	types "github.com/ledgerwatch/erigon/zk/rpcdaemon"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
)

func (api *ZkEvmAPIImpl) GetBatchSealTime(ctx context.Context, batchNumber rpc.BlockNumber) (types.ArgUint64, error) {
	lastBatchNo, err := api.BatchNumber(ctx)
	if err != nil {
		return 0, err
	}

	if batchNumber.Int64() >= int64(lastBatchNo) {
		return 0, errors.New(fmt.Sprintf("couldn't get batch number %d's seal time, error: unexpected batch. got %d, last batch should be %d", batchNumber, batchNumber, lastBatchNo))
	}

	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var lastBlockNum = uint64(0)
	lastBlockNum, err = getLastBlockInBatchNumber(tx, uint64(batchNumber.Int64()))
	if err != nil {
		return 0, err
	}

	lastBlock, err := api.GetFullBlockByNumber(ctx, rpc.BlockNumber(lastBlockNum), false)
	if err != nil {
		return 0, err
	}

	return lastBlock.Timestamp, nil
}

func (api *ZkEvmAPIImpl) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
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

func (api *ZkEvmAPIImpl) GetInternalTransactions(ctx context.Context, hash common.Hash) ([]*zktypes.InnerTx, error) {
	_, _, innerTxs, ok := api.txInfoMap.Get(hash)
	if !ok {
		return api.ethApi.GetInternalTransactions(ctx, hash)
	}

	return innerTxs, nil
}
