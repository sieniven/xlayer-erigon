package realtime

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
	realtimeTypes "github.com/ledgerwatch/erigon/zk/realtime/types"
)

const (
	// Kafka tx message cache size
	DefaultTxMsgSliceSize = 100

	// Stateless cache sizes
	DefaultStatelessBlockCacheSize = 1000
	DefaultStatelessTxCacheSize    = 10000

	// State cache size
	DefaultStateCacheSize = 1_000_000
)

type RealtimeCache struct {
	State     *PlainStateCache
	Stateless *realtimeTypes.StatelessCache
}

func NewRealtimeCache(ctx context.Context, db kv.RoDB) (*RealtimeCache, error) {
	stateCache, err := NewPlainStateCache(ctx, db, DefaultStateCacheSize)
	if err != nil {
		return nil, err
	}

	return &RealtimeCache{
		State:     stateCache,
		Stateless: realtimeTypes.NewStatelessCache(DefaultStatelessBlockCacheSize, DefaultStatelessTxCacheSize),
	}, nil
}

func (cache *RealtimeCache) Clear() {
	cache.Stateless.Clear()
	cache.State.Clear()
}
