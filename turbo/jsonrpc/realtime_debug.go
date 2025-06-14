package jsonrpc

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/zkevm/log"
)

func (api *RealtimeAPIImpl) DumpStateCache(ctx context.Context) error {
	if api == nil {
		return fmt.Errorf("api is nil")
	}

	if api.stateCache == nil {
		return fmt.Errorf("stateCache is nil")
	}

	log.Info("StateCache type", "type", reflect.TypeOf(api.stateCache).String())
	log.Info("StateCache value", "value", fmt.Sprintf("%+v", api.stateCache))

	rv := reflect.ValueOf(api.stateCache)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return fmt.Errorf("stateCache is a nil pointer")
	}

	cache, ok := api.stateCache.(*state.PlainStateCache)
	if !ok {
		return fmt.Errorf("stateCache is not of type *state.PlainStateCache, actual type: %v", reflect.TypeOf(api.stateCache))
	}

	if cache == nil {
		return fmt.Errorf("cache is nil after type assertion")
	}

	if !cache.IsReady() {
		return fmt.Errorf("cache is not ready")
	}

	if err := cache.Dump(); err != nil {
		log.Error("Failed to dump state cache", "error", err)
		return fmt.Errorf("failed to dump state cache: %v", err)
	}

	return nil
}
