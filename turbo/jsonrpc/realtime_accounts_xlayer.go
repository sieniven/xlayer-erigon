package jsonrpc

import (
	"context"
	"fmt"
	"math/big"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon/common"
)

func (api *RealtimeAPIImpl) GetBalance(ctx context.Context, address libcommon.Address) (*hexutil.Big, error) {
	acc, err := api.stateCache.ReadAccountData(address)
	if err != nil {
		return nil, fmt.Errorf("cant get a balance for account %x: %w", address.String(), err)
	}
	if acc == nil {
		// Special case - non-existent account is assumed to have zero balance
		return (*hexutil.Big)(big.NewInt(0)), nil
	}

	return (*hexutil.Big)(acc.Balance.ToBig()), nil
}

func (api *RealtimeAPIImpl) GetTransactionCount(ctx context.Context, address libcommon.Address) (*hexutil.Uint64, error) {
	acc, err := api.stateCache.ReadAccountData(address)
	if err != nil {
		return nil, fmt.Errorf("cant get a transaction count for account %x: %w", address.String(), err)
	}
	if acc == nil {
		return nil, nil
	}
	return (*hexutil.Uint64)(&acc.Nonce), nil
}

func (api *RealtimeAPIImpl) GetStorageAt(ctx context.Context, address libcommon.Address, index string) (string, error) {
	var empty []byte

	acc, err := api.stateCache.ReadAccountData(address)
	if acc == nil || err != nil {
		return hexutility.Encode(common.LeftPadBytes(empty, 32)), err
	}

	location := libcommon.HexToHash(index)
	res, err := api.stateCache.ReadAccountStorage(address, acc.Incarnation, &location)
	if err != nil {
		res = empty
	}
	return hexutility.Encode(common.LeftPadBytes(res, 32)), err
}

func (api *RealtimeAPIImpl) GetCode(ctx context.Context, address libcommon.Address) (hexutility.Bytes, error) {
	acc, err := api.stateCache.ReadAccountData(address)
	if acc == nil || err != nil {
		return hexutility.Bytes(""), nil
	}
	res, _ := api.stateCache.ReadAccountCode(address, acc.Incarnation, acc.CodeHash)
	if res == nil {
		return hexutility.Bytes(""), nil
	}
	return res, nil
}
