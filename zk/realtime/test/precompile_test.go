package test

import (
	"context"
	"testing"

	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/zk/realtime/rtclient"
	"github.com/stretchr/testify/require"
)

func TestPrecompile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)

	privateKey, err := crypto.HexToECDSA(DefaultL2AdminPrivateKey[2:])
	require.NoError(t, err)

	// Deploy precompile caller contract
	precompileCallerAddr := DeployPrecompileCallerContract(t, ctx, client)
	signedTx := SendCallPrecompileTx(t, ctx, client, privateKey, precompileCallerAddr)

	err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	txReceipt, err := client.RealtimeGetTransactionReceipt(signedTx.Hash())
	require.NoError(t, err)
	require.NotNil(t, txReceipt, "tx receipt not found")
	require.Equal(t, uint64(1), txReceipt.Status, "tx should be successful")

	// Compare state cache
	mismatches, err := client.RealtimeCompareStateCache()
	require.NoError(t, err)
	require.Empty(t, mismatches, "state cache should have no mismatches")
}
