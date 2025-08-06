package realtimeapi

import (
	"context"
	"fmt"
	"math/big"

	"errors"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/adapter/ethapi"
	"github.com/ledgerwatch/erigon/turbo/jsonrpc"
	"github.com/ledgerwatch/erigon/turbo/transactions"
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

// EstimateGas implements eth_estimateGas using realtime cache for more accurate gas estimation.
// Returns an estimate of how much gas is necessary to allow the transaction to complete.
func (api *RealtimeAPIImpl) EstimateGas(ctx context.Context, argsOrNil *ethapi.CallArgs, blockNrOrHash *rpc.BlockNumberOrHash) (hexutil.Uint64, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.APIImpl.EstimateGas(ctx, argsOrNil, blockNrOrHash)
	}

	// Parse arguments
	var args ethapi.CallArgs
	if argsOrNil != nil {
		args = *argsOrNil
	}

	// Use zero address if sender unspecified
	if args.From == nil {
		args.From = new(libcommon.Address)
	}

	// Default to pending block for realtime estimation
	bNrOrHash := rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}

	// Only support pending block for realtime gas estimation
	if bNrOrHash.BlockNumber == nil || *bNrOrHash.BlockNumber != rpc.PendingBlockNumber {
		return api.APIImpl.EstimateGas(ctx, argsOrNil, blockNrOrHash)
	}

	// Begin database transaction
	tx, err := api.APIImpl.GetDB().BeginRo(ctx)
	if err != nil {
		return api.APIImpl.EstimateGas(ctx, &args, nil)
	}
	defer tx.Rollback()

	chainConfig, err := api.APIImpl.GetChainConfig(ctx, tx)
	if err != nil {
		return api.APIImpl.EstimateGas(ctx, &args, nil)
	}

	// Get the latest pending block from realtime cache
	blockNumber, _, err := api.getBlockNumber(rpc.PendingBlockNumber)
	if err != nil {
		return api.APIImpl.EstimateGas(ctx, &args, nil)
	}

	header, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNumber)
	if !ok {
		return api.APIImpl.EstimateGas(ctx, &args, nil)
	}

	// Binary search for gas estimation
	var (
		lo     = params.TxGas - 1
		hi     uint64
		gasCap uint64
	)

	// Determine the highest gas limit
	if args.Gas != nil && uint64(*args.Gas) >= params.TxGas {
		hi = uint64(*args.Gas)
	} else {
		hi = header.GasLimit
	}

	var feeCap *big.Int
	if args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) {
		return 0, errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified")
	} else if args.GasPrice != nil {
		feeCap = args.GasPrice.ToInt()
	} else if args.MaxFeePerGas != nil {
		feeCap = args.MaxFeePerGas.ToInt()
	} else {
		feeCap = libcommon.Big0
	}

	// Check balance using realtime state cache
	if feeCap.Sign() != 0 {
		account, err := api.cacheDB.State.ReadAccountData(*args.From)
		if err != nil {
			return api.APIImpl.EstimateGas(ctx, &args, nil)
		}

		var balance *big.Int
		if account != nil {
			balance = account.Balance.ToBig()
		} else {
			balance = big.NewInt(0)
		}

		available := balance
		if args.Value != nil {
			if args.Value.ToInt().Cmp(available) >= 0 {
				return 0, errors.New("insufficient funds for transfer")
			}
			available.Sub(available, args.Value.ToInt())
		}
		allowance := new(big.Int).Div(available, feeCap)

		if allowance.IsUint64() && hi > allowance.Uint64() {
			hi = allowance.Uint64()
		}
	}

	// Cap gas limit with API gas cap
	if hi > api.APIImpl.GasCap {
		hi = api.APIImpl.GasCap
	}
	gasCap = hi

	engine := api.APIImpl.GetEngine()
	bn := rpc.BlockNumber(blockNumber)
	rpcBlockNr := rpc.BlockNumberOrHash{BlockNumber: &bn}

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) (bool, *core.ExecutionResult, error) {
		// Create temporary args with the new gas limit
		tempArgs := args
		gasVal := hexutil.Uint64(gas)
		tempArgs.Gas = &gasVal

		// Use the simpler DoCall function that doesn't require ZK database data
		result, err := transactions.DoCall(
			ctx,
			engine,
			tempArgs,
			tx,
			rpcBlockNr,
			header,
			nil, // no overrides
			gas,
			chainConfig,
			api.cacheDB.State,     // Use realtime state cache
			api.cacheDB.Stateless, // Use realtime cache as header reader
			api.APIImpl.GetEvmCallTimeout(),
		)
		if err != nil {
			if errors.Is(err, core.ErrIntrinsicGas) {
				// Special case, raise gas limit
				return true, nil, nil
			}
			return true, nil, err
		}

		return result.Failed(), result, nil
	}

	// Execute binary search to find optimal gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		failed, _, err := executable(mid)
		if err != nil {
			return api.APIImpl.EstimateGas(ctx, &args, nil)
		}
		if failed {
			lo = mid
		} else {
			hi = mid
		}
	}

	// Reject the transaction if it still fails at the highest allowance
	if hi == gasCap {
		failed, result, err := executable(hi)
		if err != nil {
			return api.APIImpl.EstimateGas(ctx, &args, nil)
		}
		if failed {
			if result != nil && result.Err != nil && result.Err.Error() != "out of gas" {
				if len(result.Revert()) > 0 {
					return 0, ethapi.NewRevertError(result)
				}
				return 0, result.Err
			}
			return 0, fmt.Errorf("gas required exceeds allowance (%d)", gasCap)
		}
	}

	return hexutil.Uint64(hi), nil
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
func (api *RealtimeAPIImpl) tryGetBlockResponseFromNumber(
	blockNum uint64,
	fullTx bool,
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

	return response, nil
}
