package jsonrpc

import (
	"context"
	"fmt"
	"math/big"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/filters"
	"github.com/ledgerwatch/erigon/rpc"
	ethapi2 "github.com/ledgerwatch/erigon/turbo/adapter/ethapi"
	realtimeCache "github.com/ledgerwatch/erigon/zk/realtime/cache"
	"github.com/ledgerwatch/erigon/zk/realtime/subscription"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
)

var (
	mockBlockHash                   = libcommon.BytesToHash([]byte{1})
	ErrRealtimeNotEnabled           = fmt.Errorf("realtime is not enabled")
	ErrRealtimeConfirmBlockNotFound = fmt.Errorf("realtime confirm block not found")
)

// RealtimeAPI is a collection of functions that are exposed in rpc only
type RealtimeAPI interface {
	// Block related (see ./realtime_blocks_xlayer.go)
	BlockNumber(ctx context.Context) (hexutil.Uint64, error)
	GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error)

	// Transaction related (see ./realtime_txs_xlayer.go)
	GetTransactionByHash(ctx context.Context, hash libcommon.Hash, includeExtraInfo *bool) (interface{}, error)
	GetRawTransactionByHash(ctx context.Context, hash libcommon.Hash) (hexutility.Bytes, error)

	// Receipt related (see ./realtime_receipts_xlayer.go)
	GetTransactionReceipt(ctx context.Context, hash libcommon.Hash) (map[string]interface{}, error)
	GetInternalTransactions(ctx context.Context, hash libcommon.Hash) ([]*zktypes.InnerTx, error)

	// Account related (see ./realtime_accounts_xlayer.go)
	GetBalance(ctx context.Context, address libcommon.Address) (*hexutil.Big, error)
	GetTransactionCount(ctx context.Context, address libcommon.Address) (*hexutil.Uint64, error)
	GetCode(ctx context.Context, address libcommon.Address) (hexutility.Bytes, error)
	GetStorageAt(ctx context.Context, address libcommon.Address, index string) (string, error)

	// Sending related (see ./realtime_call_xlayer.go)
	Call(ctx context.Context, args ethapi2.CallArgs, overrides *ethapi2.StateOverrides) (hexutility.Bytes, error)

	// Debug related (see ./realtime_debug.go)
	DumpStateCache(ctx context.Context) error
}

type RealtimeSubscriptionAPI interface {
	// WebSocket subscription methods
	RealtimeTransactions(ctx context.Context, fullTx, includeExtraInfo *bool) (*rpc.Subscription, error)
	Logs(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error)
}

// RealtimeAPIImpl is implementation of the RealtimeAPI interface
type RealtimeAPIImpl struct {
	cacheDB       *realtimeCache.RealtimeCache
	subService    *subscription.RealtimeSubscription
	ethApi        *APIImpl
	enableFlag    bool
	cacheDumpPath string
}

// NewRealtimeAPI returns RealtimeAPIImpl instance
func NewRealtimeAPI(
	cacheDB *realtimeCache.RealtimeCache,
	subService *subscription.RealtimeSubscription,
	base *APIImpl,
	enableFlag bool,
	cacheDumpPath string,
) *RealtimeAPIImpl {

	return &RealtimeAPIImpl{
		cacheDB:       cacheDB,
		subService:    subService,
		ethApi:        base,
		enableFlag:    enableFlag,
		cacheDumpPath: cacheDumpPath,
	}
}

func (api *RealtimeAPIImpl) getBlockNumber(blockNr rpc.BlockNumber) (uint64, bool, error) {
	if !api.enableFlag {
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
func newRPCTransaction_realtime(tx types.Transaction, blockNumber uint64, index uint64, baseFee *big.Int) *RPCTransaction {
	result := NewRPCTransaction(tx, mockBlockHash, blockNumber, index, baseFee)
	result.BlockHash = &libcommon.Hash{}
	return result
}
