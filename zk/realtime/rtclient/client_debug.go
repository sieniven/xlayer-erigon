package rtclient

import (
	"fmt"

	"github.com/ledgerwatch/erigon/zkevm/jsonrpc/client"
)

// RealtimeDumpStateCache dumps the state cache
func (rc *RealtimeClient) RealtimeDumpCache() error {
	response, err := client.JSONRPCCall(rc.url, "eth_debugDumpRealtimeCache")
	if err != nil {
		return err
	}
	if response.Error != nil {
		return fmt.Errorf("%d - %s", response.Error.Code, response.Error.Message)
	}

	return nil
}
