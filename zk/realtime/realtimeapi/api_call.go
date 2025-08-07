package realtimeapi

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/adapter/ethapi"
	ethapi2 "github.com/ledgerwatch/erigon/turbo/adapter/ethapi"
	"github.com/ledgerwatch/erigon/turbo/transactions"
)

// Call implements realtime_call.
// Executes a new message call immediately without creating a transaction on the block chain.
// Note that realtime API only supports execution on the latest block.
func (api *RealtimeAPIImpl) Call(ctx context.Context, args ethapi2.CallArgs, blockNrOrHash rpc.BlockNumberOrHash, overrides *ethapi2.StateOverrides) (hexutility.Bytes, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.APIImpl.Call(ctx, args, blockNrOrHash, overrides)
	}

	reader, blockNumber, err := api.createStateReader(&blockNrOrHash)
	if err != nil || reader == nil {
		return api.APIImpl.Call(ctx, args, blockNrOrHash, overrides)
	}

	return api.doRealtimeCall(ctx, args, overrides, blockNumber, reader)
}

func (api *RealtimeAPIImpl) doRealtimeCall(ctx context.Context, args ethapi2.CallArgs, overrides *ethapi2.StateOverrides, blockNumber uint64, reader state.StateReader) (hexutility.Bytes, error) {
	tx, err := api.APIImpl.GetDB().BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	chainConfig, err := api.APIImpl.GetChainConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	engine := api.APIImpl.GetEngine()

	if args.Gas == nil || uint64(*args.Gas) == 0 {
		args.Gas = (*hexutil.Uint64)(&api.APIImpl.GasCap)
	}

	header, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNumber)
	if !ok {
		return nil, fmt.Errorf("header not found for block number %d", blockNumber)
	}

	bn := rpc.BlockNumber(blockNumber)
	rpcBlockNr := rpc.BlockNumberOrHash{BlockNumber: &bn}
	result, err := transactions.DoCall(ctx, engine, args, tx, rpcBlockNr, header, overrides, api.APIImpl.GasCap, chainConfig, reader, api.cacheDB.Stateless, api.APIImpl.GetEvmCallTimeout())
	if err != nil {
		return nil, err
	}

	if len(result.ReturnData) > api.APIImpl.ReturnDataLimit {
		return nil, fmt.Errorf("call returned result on length %d exceeding --rpc.returndata.limit %d", len(result.ReturnData), api.APIImpl.ReturnDataLimit)
	}

	// If the result contains a revert reason, try to unpack and return it.
	if len(result.Revert()) > 0 {
		return nil, ethapi2.NewRevertError(result)
	}

	return result.Return(), result.Err
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
