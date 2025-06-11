package jsonrpc

import (
	"context"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/rpc"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
)

// RealtimeAPI is a collection of functions that are exposed in rpc only
type RealtimeAPI interface {
	// // Block related (see ./realtime_blocks_xlayer.go)
	BlockNumber(ctx context.Context) (hexutil.Uint64, error)
	GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error)

	// // Transaction related (see ./realtime_txs_xlayer.go)
	// GetTransactionByHash(ctx context.Context, hash common.Hash, includeExtraInfo *bool) (interface{}, error)
	// GetRawTransactionByHash(ctx context.Context, hash common.Hash) (hexutility.Bytes, error)

	// Receipt related (see ./realtime_receipts_xlayer.go)
	GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error)
	GetInternalTransactions(ctx context.Context, hash common.Hash) ([]*zktypes.InnerTx, error)

	// // Account related (see ./realtime_accounts_xlayer.go)
	// GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Big, error)
	// GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash *rpc.BlockNumberOrHash) (*hexutil.Uint64, error)
	// GetStorageAt(ctx context.Context, address common.Address, index string, blockNrOrHash rpc.BlockNumberOrHash) (string, error)
	// GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (hexutility.Bytes, error)

	// // Sending related (see ./realtime_call_xlayer.go)
	// Call(ctx context.Context, args ethapi2.CallArgs, blockNrOrHash rpc.BlockNumberOrHash, overrides *ethapi2.StateOverrides) (hexutility.Bytes, error)
}

// RealtimeAPIImpl is implementation of the RealtimeAPI interface
type RealtimeAPIImpl struct {
	ethApi         *APIImpl
	stateCache     state.StateReader
	statelessCache *zktypes.StatelessCache
}

// NewRealtimeAPI returns RealtimeAPIImpl instance
func NewRealtimeAPI(
	base *APIImpl,
	stateCache state.StateReader,
	statelessCache *zktypes.StatelessCache,
) *RealtimeAPIImpl {

	return &RealtimeAPIImpl{
		ethApi:         base,
		stateCache:     stateCache,
		statelessCache: statelessCache,
	}
}

func (api *RealtimeAPIImpl) GetBlockNumber(blockNr rpc.BlockNumber) (uint64, bool, error) {
	currentBlockNumber := api.statelessCache.GetHeight()
	if currentBlockNumber == 0 {
		return 0, false, fmt.Errorf("no block number found in stateless cache")
	}

	switch blockNr {
	case rpc.LatestBlockNumber:
		return currentBlockNumber, true, nil
	case rpc.EarliestBlockNumber:
		return 0, false, nil
	case rpc.FinalizedBlockNumber:
		return currentBlockNumber, true, nil
	case rpc.SafeBlockNumber:
		return currentBlockNumber, true, nil
	case rpc.PendingBlockNumber:
		return currentBlockNumber, true, nil
	case rpc.LatestExecutedBlockNumber:
		return currentBlockNumber, true, nil
	default:
		blockNumber := uint64(blockNr.Int64())
		if blockNumber > currentBlockNumber {
			return 0, false, fmt.Errorf("block with number %d not found", blockNumber)
		}
		return blockNumber, blockNumber == currentBlockNumber, nil
	}
}
