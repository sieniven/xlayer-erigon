package realtimeapi

import (
	"fmt"
	"math/big"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/adapter/ethapi"
	"github.com/ledgerwatch/erigon/turbo/jsonrpc"
	realtimeCache "github.com/ledgerwatch/erigon/zk/realtime/cache"
	"github.com/ledgerwatch/erigon/zk/realtime/subscription"
)

var (
	MockBlockHash         = libcommon.BytesToHash([]byte{1})
	EmptyBlockHash        = libcommon.Hash{}
	ErrRealtimeNotEnabled = fmt.Errorf("realtime is not enabled")
)

// RealtimeAPIImpl is implementation of the RealtimeAPI interface
type RealtimeAPIImpl struct {
	jsonrpc.APIImpl
	cacheDB    *realtimeCache.RealtimeCache
	subService *subscription.RealtimeSubscription
}

// NewRealtimeAPIImpl returns RealtimeAPIImpl instance
func NewRealtimeAPIImpl(
	base *jsonrpc.APIImpl,
	cacheDB *realtimeCache.RealtimeCache,
	subService *subscription.RealtimeSubscription,
) *RealtimeAPIImpl {

	return &RealtimeAPIImpl{
		APIImpl:    *base,
		cacheDB:    cacheDB,
		subService: subService,
	}
}

func NewRealtimeAPI(
	base *jsonrpc.APIImpl,
	cacheDB *realtimeCache.RealtimeCache,
	subService *subscription.RealtimeSubscription,
) interface{} {
	return NewRealtimeAPIImpl(base, cacheDB, subService)
}

func (api *RealtimeAPIImpl) getBlockNumber(blockNr rpc.BlockNumber) (uint64, bool, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return 0, false, ErrRealtimeNotEnabled
	}

	confirmHeight := api.cacheDB.GetHighestConfirmHeight()
	if confirmHeight == 0 {
		return 0, false, fmt.Errorf("no block number found in stateless cache")
	}

	switch blockNr {
	case rpc.LatestBlockNumber:
		return confirmHeight, true, nil
	case rpc.EarliestBlockNumber:
		// Unsupported
		return 0, false, fmt.Errorf("earliest block number is not supported")
	case rpc.FinalizedBlockNumber:
		return confirmHeight, true, nil
	case rpc.SafeBlockNumber:
		return confirmHeight, true, nil
	case rpc.PendingBlockNumber:
		pendingHeight := api.cacheDB.GetHighestPendingHeight()
		if pendingHeight == 0 {
			return 0, false, fmt.Errorf("no block number found in stateless cache")
		}
		return pendingHeight, true, nil
	case rpc.LatestExecutedBlockNumber:
		return confirmHeight, true, nil
	default:
		blockNumber := uint64(blockNr.Int64())
		if blockNumber > confirmHeight {
			return 0, false, fmt.Errorf("block with number %d not found", blockNumber)
		}
		return blockNumber, blockNumber == confirmHeight, nil
	}
}

// newRPCTransaction_realtime returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
// Note that realtime API do not support blockHash.
func newRPCTransaction_realtime(tx types.Transaction, txblockhash libcommon.Hash, blockNumber uint64, index uint64, baseFee *big.Int) *jsonrpc.RPCTransaction {
	blockhash := txblockhash
	if blockhash == EmptyBlockHash {
		blockhash = MockBlockHash
	}

	result := jsonrpc.NewRPCTransaction(tx, blockhash, blockNumber, index, baseFee)
	result.BlockHash = &txblockhash
	return result
}

// formatBlockResponse creates a formatted block response from cache data
// This utility function consolidates the block formatting logic used by both
// GetBlockByNumber and GetBlockByHash methods
func (api *RealtimeAPIImpl) formatBlockResponse(
	blockNum uint64,
	fullTx bool,
	isPending bool,
) (map[string]interface{}, error) {
	header, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNum)
	if !ok {
		return nil, fmt.Errorf("header not found for block %d", blockNum)
	}

	var transactions []types.Transaction
	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if ok {
		for _, txHash := range txHashes {
			if tx, _, _, _, exists := api.cacheDB.Stateless.GetTxInfo(txHash); exists {
				transactions = append(transactions, tx)
			} else {
				return nil, fmt.Errorf("transaction %s not found in cache", txHash.Hex())
			}
		}
	}

	block := types.NewBlockWithHeader(header).WithBody(transactions, nil)

	additionalFields := map[string]interface{}{
		"totalDifficulty": (*hexutil.Big)(header.Difficulty),
	}

	response, err := ethapi.RPCMarshalBlockEx(block, true, fullTx, nil, libcommon.Hash{}, additionalFields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block: %w", err)
	}

	// Handle pending block special case
	if isPending {
		for _, field := range []string{"hash", "nonce", "miner"} {
			response[field] = nil
		}
	}

	return response, nil
}
