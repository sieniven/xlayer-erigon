package jsonrpc

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon/core/types/accounts"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/rpchelper"
	"github.com/ledgerwatch/erigon/zkevm/log"
)

func (api *RealtimeAPIImpl) DumpCache(ctx context.Context) error {
	if api == nil || !api.enableFlag {
		return ErrRealtimeNotEnabled
	}

	if api.cacheDB == nil {
		return fmt.Errorf("stateCache is nil")
	}

	rv := reflect.ValueOf(api.cacheDB.State)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return fmt.Errorf("stateCache is a nil pointer")
	}

	if err := api.cacheDB.DumpToFile(); err != nil {
		log.Error("[Realtime] Failed to dump state cache", "error", err)
		return fmt.Errorf("failed to dump state cache: %v", err)
	}

	return nil
}

func (api *RealtimeAPIImpl) CompareStateCache(ctx context.Context) error {
	if api == nil || !api.enableFlag {
		return ErrRealtimeNotEnabled
	}

	if api.cacheDB == nil || api.cacheDB.State == nil {
		return fmt.Errorf("stateCache is nil")
	}

	rv := reflect.ValueOf(api.cacheDB.State)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return fmt.Errorf("stateCache is a nil pointer")
	}

	tx, err1 := api.ethApi.db.BeginRo(ctx)
	if err1 != nil {
		return fmt.Errorf("compareStateCache cannot open tx: %w", err1)
	}
	defer tx.Rollback()
	blockNumber := rpc.LatestBlockNumber
	reader, err := rpchelper.CreateStateReader(ctx, tx, rpc.BlockNumberOrHash{BlockNumber: &blockNumber}, 0, api.ethApi.filters, api.ethApi.stateCache, api.ethApi.historyV3(tx), "")
	if err != nil {
		return err
	}

	// Use WithAccountCache to ensure thread-safe access during the entire comparison
	return api.cacheDB.State.WithAccountCache(func(accountCache map[libcommon.Address]*accounts.Account) error {
		var mismatches []string

		for addr, accCache := range accountCache {
			log.Info("[Realtime] Comparing account", "address", addr.String())
			accDb, err := reader.ReadAccountData(addr)
			if err != nil {
				log.Error("[Realtime] Failed to read account from database", "address", addr.String(), "error", err)
				mismatches = append(mismatches, fmt.Sprintf("failed to read account %s: %v", addr.String(), err))
				continue
			}
			if accDb == nil {
				log.Error("[Realtime] Account not found in database", "address", addr.String())
				mismatches = append(mismatches, fmt.Sprintf("account %s not found in database", addr.String()))
				continue
			}

			if accCache.Nonce != accDb.Nonce {
				log.Error("[Realtime] Nonce mismatch", "address", addr.String(), "cache_nonce", accCache.Nonce, "db_nonce", accDb.Nonce)
				mismatches = append(mismatches, fmt.Sprintf("nonce mismatch for account %s: %d != %d", addr.String(), accCache.Nonce, accDb.Nonce))
			}

			if accCache.Balance.Cmp(&accDb.Balance) != 0 {
				log.Error("[Realtime] Balance mismatch", "address", addr.String(), "cache_balance", (*hexutil.Big)(accCache.Balance.ToBig()), "db_balance", (*hexutil.Big)(accDb.Balance.ToBig()))
				mismatches = append(mismatches, fmt.Sprintf("balance mismatch for account %s: %v != %v", addr.String(), accCache.Balance.ToBig(), accDb.Balance.ToBig()))
			}

			if accCache.Root != accDb.Root {
				log.Error("[Realtime] Root mismatch", "address", addr.String(), "cache_root", accCache.Root, "db_root", accDb.Root)
				mismatches = append(mismatches, fmt.Sprintf("root mismatch for account %s: %s != %s", addr.String(), accCache.Root.String(), accDb.Root.String()))
			}

			if accCache.CodeHash != accDb.CodeHash {
				log.Error("[Realtime] Code hash mismatch", "address", addr.String(), "cache_code_hash", accCache.CodeHash, "db_code_hash", accDb.CodeHash)
				mismatches = append(mismatches, fmt.Sprintf("code hash mismatch for account %s: %s != %s", addr.String(), accCache.CodeHash.String(), accDb.CodeHash.String()))
			}
		}

		if len(mismatches) > 0 {
			return fmt.Errorf("found %d mismatches between cache and database: %s", len(mismatches), strings.Join(mismatches, "; "))
		}

		log.Info("[Realtime] State cache comparison completed successfully", "total_accounts", len(accountCache))
		return nil
	})
}
