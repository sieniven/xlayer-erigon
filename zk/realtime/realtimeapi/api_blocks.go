package realtimeapi

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon/rpc"
)

func (api *RealtimeAPIImpl) BlockNumber(ctx context.Context, tag *RealtimeTag) (hexutil.Uint64, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.APIImpl.BlockNumber(ctx)
	}

	if tag == nil {
		return api.APIImpl.BlockNumber(ctx)
	}

	blockNumber, _, err := api.getBlockNumber(rpc.BlockNumber(*tag))
	if err != nil {
		// Do not redirect to default eth api as block number with tag is custom for realtime
		return hexutil.Uint64(0), ErrRealtimeNotEnabled
	}
	return hexutil.Uint64(blockNumber), nil
}

func (api *RealtimeAPIImpl) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.APIImpl.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	blockNum, _, err := api.getBlockNumber(blockNr)
	if err != nil {
		return api.APIImpl.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	_, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNum)
	if !ok {
		return api.APIImpl.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	txs, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.APIImpl.GetBlockTransactionCountByNumber(ctx, blockNr)
	}
	numOfTx := hexutil.Uint(len(txs))
	return &numOfTx, nil
}
