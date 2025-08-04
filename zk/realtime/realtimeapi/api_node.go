package realtimeapi

import (
	"context"
	"fmt"
)

// Returns the status on whether the RT feature is enabled (when tag provided)
func (api *RealtimeAPIImpl) RealtimeEnabled(ctx context.Context, tag RealtimeTag) (bool, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return false, nil
	}
	if tag != Pending {
		return false, fmt.Errorf("invalid tag, only 'pending' is supported")
	}
	return true, nil
}
