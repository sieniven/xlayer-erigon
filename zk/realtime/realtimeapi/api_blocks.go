package realtimeapi

import (
	"context"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/adapter/ethapi"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
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
		return hexutil.Uint64(0), err
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

func (api *RealtimeAPIImpl) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx *bool) (map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.APIImpl.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	if fullTx == nil {
		fullTx = new(bool)
	}

	blockNum, _, err := api.getBlockNumber(blockNr)
	if err != nil {
		return api.APIImpl.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	header, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNum)
	if !ok {
		return api.APIImpl.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.APIImpl.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	var transactions []types.Transaction
	for _, txHash := range txHashes {
		if tx, _, _, _, exists := api.cacheDB.Stateless.GetTxInfo(txHash); exists {
			transactions = append(transactions, tx)
		} else {
			return api.APIImpl.GetBlockByNumber(ctx, blockNr, fullTx)
		}
	}

	block := types.NewBlockWithHeader(header).WithBody(transactions, nil)

	additionalFields := map[string]interface{}{
		"totalDifficulty": (*hexutil.Big)(header.Difficulty),
	}

	response, err := ethapi.RPCMarshalBlockEx(block, true, *fullTx, nil, libcommon.Hash{}, additionalFields)
	if err != nil {
		return api.APIImpl.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	if blockNr == rpc.PendingBlockNumber {
		for _, field := range []string{"hash", "nonce", "miner"} {
			response[field] = nil
		}
	}

	return response, nil
}

func (api *RealtimeAPIImpl) GetBlockByHash(ctx context.Context, numberOrHash rpc.BlockNumberOrHash, fullTx *bool) (map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.APIImpl.GetBlockByHash(ctx, numberOrHash, fullTx)
	}

	if fullTx == nil {
		fullTx = new(bool)
	}

	if numberOrHash.BlockNumber != nil {
		return api.GetBlockByNumber(ctx, *numberOrHash.BlockNumber, fullTx)
	}

	if numberOrHash.BlockHash != nil {
		targetHash := *numberOrHash.BlockHash

		header, _, _, ok := api.cacheDB.Stateless.GetHeaderByHash(targetHash)
		if !ok {
			return api.APIImpl.GetBlockByHash(ctx, numberOrHash, fullTx)
		}

		blockNum, found := api.cacheDB.Stateless.GetBlockNumberByHash(targetHash)
		if !found {
			return api.APIImpl.GetBlockByHash(ctx, numberOrHash, fullTx)
		}

		txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
		if !ok {
			return api.APIImpl.GetBlockByHash(ctx, numberOrHash, fullTx)
		}

		var transactions []types.Transaction
		for _, txHash := range txHashes {
			if tx, _, _, _, exists := api.cacheDB.Stateless.GetTxInfo(txHash); exists {
				transactions = append(transactions, tx)
			} else {
				return api.APIImpl.GetBlockByHash(ctx, numberOrHash, fullTx)
			}
		}

		block := types.NewBlockWithHeader(header).WithBody(transactions, nil)

		additionalFields := map[string]interface{}{
			"totalDifficulty": (*hexutil.Big)(header.Difficulty),
		}

		response, err := ethapi.RPCMarshalBlockEx(block, true, *fullTx, nil, libcommon.Hash{}, additionalFields)
		if err != nil {
			return api.APIImpl.GetBlockByHash(ctx, numberOrHash, fullTx)
		}

		return response, nil
	}

	return api.APIImpl.GetBlockByHash(ctx, numberOrHash, fullTx)
}

func (api *RealtimeAPIImpl) GetBlockTransactionCountByHash(ctx context.Context, blockHash libcommon.Hash) (*hexutil.Uint, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.APIImpl.GetBlockTransactionCountByHash(ctx, blockHash)
	}

	blockNum, found := api.cacheDB.Stateless.GetBlockNumberByHash(blockHash)
	if !found {
		return api.APIImpl.GetBlockTransactionCountByHash(ctx, blockHash)
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.APIImpl.GetBlockTransactionCountByHash(ctx, blockHash)
	}

	numOfTx := hexutil.Uint(len(txHashes))
	return &numOfTx, nil
}

func (api *RealtimeAPIImpl) GetBlockInternalTransactions(ctx context.Context, blockNr rpc.BlockNumber) (map[libcommon.Hash][]*zktypes.InnerTx, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.APIImpl.GetBlockInternalTransactions(ctx, blockNr)
	}

	blockNum, _, err := api.getBlockNumber(blockNr)
	if err != nil {
		return api.APIImpl.GetBlockInternalTransactions(ctx, blockNr)
	}

	_, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNum)
	if !ok {
		return api.APIImpl.GetBlockInternalTransactions(ctx, blockNr)
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.APIImpl.GetBlockInternalTransactions(ctx, blockNr)
	}

	result := make(map[libcommon.Hash][]*zktypes.InnerTx)

	for _, txHash := range txHashes {
		_, _, _, innerTxs, exists := api.cacheDB.Stateless.GetTxInfo(txHash)
		if !exists {
			return api.APIImpl.GetBlockInternalTransactions(ctx, blockNr)
		}
		result[txHash] = innerTxs
	}

	return result, nil
}
