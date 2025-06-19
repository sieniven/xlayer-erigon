package jsonrpc

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ledgerwatch/erigon/zkevm/log"
)

func (api *RealtimeAPIImpl) DumpStateCache(ctx context.Context) error {
	if api == nil {
		return fmt.Errorf("api is nil")
	}

	if api.cacheDB == nil || api.cacheDB.State == nil {
		return fmt.Errorf("stateCache is nil")
	}

	rv := reflect.ValueOf(api.cacheDB.State)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return fmt.Errorf("stateCache is a nil pointer")
	}

	if err := api.cacheDB.State.Dump(); err != nil {
		log.Error("[Realtime] Failed to dump state cache", "error", err)
		return fmt.Errorf("failed to dump state cache: %v", err)
	}

	return nil
}
