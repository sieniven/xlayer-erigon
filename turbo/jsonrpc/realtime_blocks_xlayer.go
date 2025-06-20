package jsonrpc

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon/rpc"
)

// BlockNumber implements realtime_blockNumber.
// Returns the block number of the most recent confirmed block.
func (api *RealtimeAPIImpl) BlockNumber(ctx context.Context) (hexutil.Uint64, error) {
	blockNumber, _, err := api.getBlockNumber(rpc.LatestBlockNumber)
	if err != nil {
		return api.ethApi.BlockNumber(ctx)
	}
	return hexutil.Uint64(blockNumber), nil
}

// GetBlockTransactionCountByNumber implements realtime_getBlockTransactionCountByNumber.
// Returns the number of transactions in a block given the block's block number.
func (api *RealtimeAPIImpl) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error) {
	blockNum, _, err := api.getBlockNumber(blockNr)
	if err != nil {
		return api.ethApi.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	txs, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.ethApi.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	numOfTx := hexutil.Uint(len(txs))

	return &numOfTx, nil
}
