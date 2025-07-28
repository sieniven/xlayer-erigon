package realtimeapi

import (
	"context"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon/rpc"
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

	if blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == rpc.PendingBlockNumber {
		// Realtime supported only for pending tags
		return api.doRealtimeCall(ctx, args, overrides)
	}

	return api.APIImpl.Call(ctx, args, blockNrOrHash, overrides)
}

func (api *RealtimeAPIImpl) doRealtimeCall(ctx context.Context, args ethapi2.CallArgs, overrides *ethapi2.StateOverrides) (hexutility.Bytes, error) {
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

	blockNumber, _, err := api.getBlockNumber(rpc.PendingBlockNumber)
	if err != nil {
		return nil, err
	}

	header, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNumber)
	if !ok {
		return nil, fmt.Errorf("header not found for block number %d", blockNumber)
	}

	bn := rpc.BlockNumber(blockNumber)
	rpcBlockNr := rpc.BlockNumberOrHash{BlockNumber: &bn}
	result, err := transactions.DoCall(ctx, engine, args, tx, rpcBlockNr, header, overrides, api.APIImpl.GasCap, chainConfig, api.cacheDB.State, api.cacheDB.Stateless, api.APIImpl.GetEvmCallTimeout())
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
