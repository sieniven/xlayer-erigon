package realtimeapi

import (
	"context"
	"fmt"

	"github.com/ledgerwatch/erigon/zkevm/log"
)

func (api *RealtimeAPIImpl) DebugDumpRealtimeCache(ctx context.Context) error {
	if !api.enableFlag || api.cacheDB == nil {
		// Custom for realtime
		return ErrRealtimeNotEnabled
	}

	if err := api.cacheDB.DebugDumpToFile(); err != nil {
		log.Error("[Realtime] Failed to dump state cache", "error", err)
		return fmt.Errorf("failed to dump state cache: %v", err)
	}

	return nil
}

func (api *RealtimeAPIImpl) DebugCompareRealtimeStateCache(ctx context.Context) (*RealtimeDebugResult, error) {
	if !api.enableFlag || api.cacheDB == nil || api.cacheDB.State == nil {
		// Custom for realtime
		return nil, ErrRealtimeNotEnabled
	}

	reader, tx, err := api.APIImpl.CreateLatestStateReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("compareStateCache cannot create latest state reader: %w", err)
	}
	defer tx.Rollback()

	return &RealtimeDebugResult{
		ConfirmHeight:   api.cacheDB.GetHighestConfirmHeight(),
		ExecutionHeight: api.cacheDB.GetExecutionHeight(),
		Mismatches:      api.cacheDB.State.DebugCompare(reader),
	}, nil
}
