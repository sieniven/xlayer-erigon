package types

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/state"
	realtimeTypes "github.com/ledgerwatch/erigon/zk/realtime/types"
)

type RealtimeCache struct {
	State     *state.PlainStateCache
	Stateless *realtimeTypes.StatelessCache
}

func NewRealtimeCache(ctx context.Context, db kv.RoDB) (*RealtimeCache, error) {
	stateCache, err := state.NewPlainStateCache(ctx, db)
	if err != nil {
		return nil, err
	}

	return &RealtimeCache{
		State:     stateCache,
		Stateless: realtimeTypes.NewStatelessCache(),
	}, nil
}
