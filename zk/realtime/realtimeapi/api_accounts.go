package realtimeapi

import (
	"context"
	"fmt"
	"math/big"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/rpc"
)

func (api *RealtimeAPIImpl) GetBalance(ctx context.Context, address libcommon.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Big, error) {
	if !api.enableFlag || api.cacheDB == nil {
		return api.APIImpl.GetBalance(ctx, address, blockNrOrHash)
	}

	if blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == rpc.PendingBlockNumber {
		// Realtime supported only for pending tags
		acc, err := api.cacheDB.State.ReadAccountData(address)
		if err != nil {
			return nil, fmt.Errorf("cant get a balance for account %x: %w", address.String(), err)
		}
		if acc == nil {
			// Special case - non-existent account is assumed to have zero balance
			return (*hexutil.Big)(big.NewInt(0)), nil
		}

		return (*hexutil.Big)(acc.Balance.ToBig()), nil
	}

	return api.APIImpl.GetBalance(ctx, address, blockNrOrHash)
}

func (api *RealtimeAPIImpl) GetTransactionCount(ctx context.Context, address libcommon.Address, blockNrOrHash *rpc.BlockNumberOrHash) (*hexutil.Uint64, error) {
	if !api.enableFlag || api.cacheDB == nil {
		return api.APIImpl.GetTransactionCount(ctx, address, blockNrOrHash)
	}

	if blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == rpc.PendingBlockNumber {
		// Realtime supported only for pending tags
		ethNonce, err := api.APIImpl.GetTransactionCount(ctx, address, blockNrOrHash)
		if err != nil {
			ethNonce = nil
		}

		var cacheNonce *hexutil.Uint64
		acc, err := api.cacheDB.State.ReadAccountData(address)
		if err != nil {
			cacheNonce = nil
		} else if acc != nil {
			nonce := hexutil.Uint64(acc.Nonce)
			cacheNonce = &nonce
		}

		if ethNonce == nil && cacheNonce == nil {
			return nil, fmt.Errorf("failed to get transaction count for account %x from both sources", address)
		}

		if ethNonce == nil {
			return cacheNonce, nil
		}
		if cacheNonce == nil {
			return ethNonce, nil
		}

		if *ethNonce > *cacheNonce {
			return ethNonce, nil
		}
		return cacheNonce, nil
	}

	return api.APIImpl.GetTransactionCount(ctx, address, blockNrOrHash)
}

func (api *RealtimeAPIImpl) GetCode(ctx context.Context, address libcommon.Address, blockNrOrHash rpc.BlockNumberOrHash) (hexutility.Bytes, error) {
	if !api.enableFlag || api.cacheDB == nil {
		return api.APIImpl.GetCode(ctx, address, blockNrOrHash)
	}

	if blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == rpc.PendingBlockNumber {
		// Realtime supported only for pending tags
		acc, err := api.cacheDB.State.ReadAccountData(address)
		if acc == nil || err != nil {
			return hexutility.Bytes(""), nil
		}
		res, _ := api.cacheDB.State.ReadAccountCode(address, acc.Incarnation, acc.CodeHash)
		if res == nil {
			return hexutility.Bytes(""), nil
		}
		return res, nil
	}

	return api.APIImpl.GetCode(ctx, address, blockNrOrHash)
}

func (api *RealtimeAPIImpl) GetStorageAt(ctx context.Context, address libcommon.Address, index string, blockNrOrHash rpc.BlockNumberOrHash) (string, error) {
	if !api.enableFlag || api.cacheDB == nil {
		return api.APIImpl.GetStorageAt(ctx, address, index, blockNrOrHash)
	}

	if blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == rpc.PendingBlockNumber {
		// Realtime supported only for pending tags
		var empty []byte

		acc, err := api.cacheDB.State.ReadAccountData(address)
		if acc == nil || err != nil {
			return hexutility.Encode(common.LeftPadBytes(empty, 32)), err
		}

		location := libcommon.HexToHash(index)
		res, err := api.cacheDB.State.ReadAccountStorage(address, acc.Incarnation, &location)
		if err != nil {
			res = empty
		}
		return hexutility.Encode(common.LeftPadBytes(res, 32)), err
	}

	return api.APIImpl.GetStorageAt(ctx, address, index, blockNrOrHash)
}
