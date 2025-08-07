//go:build !skip_comparison_realtime
// +build !skip_comparison_realtime

package test

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/zk/realtime/rtclient"
	rpcTypes "github.com/ledgerwatch/erigon/zk/rpcdaemon"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
	"github.com/ledgerwatch/erigon/zkevm/encoding"
	"github.com/ledgerwatch/erigon/zkevm/log"
	log3 "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"
)

// TestRealtimeComparison is the main test function that compares various RPC methods
// between realtime and non-realtime enabled nodes to ensure output is identical
func TestRealtimeComparison(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)

	// Create shared RPC client for direct JSON RPC calls to non-realtime node
	nonRealtimeRPCClient, err := rpc.Dial(DefaultL2NetworkNoRealtimeURL, log3.Root())
	require.NoError(t, err)
	defer nonRealtimeRPCClient.Close()

	fromAddress := common.HexToAddress(DefaultL2AdminAddress)
	log.Info(fmt.Sprintf("Sender: %s", fromAddress))

	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	txHash := transToken(t, context.Background(), client, uint256.NewInt(encoding.Gwei), testAddress.String())
	txHashCommon := common.HexToHash(txHash)
	time.Sleep(1 * time.Second)

	log.Info("Starting realtime comparison test", "realtimeURL", DefaultL2NetworkRealtimeURL, "nonRealtimeURL", DefaultL2NetworkNoRealtimeURL)

	t.Run("getBlockByNumber", func(t *testing.T) {
		log.Info("Testing getBlockByNumber comparison")

		// Test blocks: latest, 1, 10
		testBlocks := []string{"latest", "0x1", "0xa"}
		allPassed := true

		for _, blockParam := range testBlocks {
			log.Info(fmt.Sprintf("Testing block: %v", blockParam))

			blockNumber, err := convertBlockParam(client, blockParam)
			if err != nil {
				t.Errorf("Failed to convert block parameter %v: %v", blockParam, err)
				allPassed = false
				continue
			}

			// Get block from realtime node
			realtimeBlock, err := client.RealtimeGetBlockByNumber(blockNumber)
			if err != nil {
				t.Errorf("Failed to get block from realtime node for %v: %v", blockParam, err)
				allPassed = false
				continue
			}

			// Make direct RPC call to non-realtime node to get JSON response
			var nonRealtimeMap map[string]interface{}
			err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeMap, "eth_getBlockByNumber", blockParam, true)
			if err != nil {
				t.Errorf("Failed to get block from non-realtime node for %v: %v", blockParam, err)
				allPassed = false
				continue
			}

			// Compare the results
			err = CompareBlockResponses(realtimeBlock, nonRealtimeMap, fmt.Sprintf("block_%v", blockParam))
			if err != nil {
				t.Errorf("Block responses differ for %v: %v", blockParam, err)
				allPassed = false
			}
		}

		require.True(t, allPassed, "getBlockByNumber test failed - some scenarios did not pass")
	})

	t.Run("getBlockByHash", func(t *testing.T) {
		allPassed := true

		// Test blocks: pending, latest, 1, 10
		testBlocks := []string{"pending", "latest", "0x1", "0xa"}

		for _, blockParam := range testBlocks {
			log.Info(fmt.Sprintf("Getting block %v by number first to extract hash", blockParam))

			blockNumber, err := convertBlockParam(client, blockParam)
			if err != nil {
				t.Logf("Failed to convert block parameter %v: %v", blockParam, err)
				continue
			}

			blockByNumber, err := client.RealtimeGetBlockByNumber(blockNumber)
			if err != nil {
				t.Logf("Could not get block %v by number: %v", blockParam, err)
				continue
			}

			blockHash, ok := extractBlockHash(blockByNumber, blockParam)
			if !ok {
				t.Logf("Block %v does not have a valid hash", blockParam)
				continue
			}
			log.Info(fmt.Sprintf("Comparing block %v by hash: %s", blockParam, blockHash.Hex()))

			// Get block from realtime node
			realtimeBlock, err := client.RealtimeGetBlockByHash(blockHash, true)
			if err != nil {
				t.Errorf("Failed to get block from realtime node for %v: %v", blockParam, err)
				allPassed = false
				continue
			}

			// Make direct RPC call to non-realtime node to get JSON response
			var nonRealtimeMap map[string]interface{}
			err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeMap, "eth_getBlockByHash", blockHash, true)
			if err != nil {
				t.Errorf("Failed to get block from non-realtime node for %v: %v", blockParam, err)
				allPassed = false
				continue
			}

			// Compare the results
			err = CompareBlockResponses(realtimeBlock, nonRealtimeMap, fmt.Sprintf("block_%v_hash", blockParam))
			if err != nil {
				t.Errorf("Block responses differ for %v hash %s: %v", blockParam, blockHash.Hex(), err)
				allPassed = false
			}
		}

		require.True(t, allPassed, "getBlockByHash test failed - some scenarios did not pass")
	})

	t.Run("getBlockTransactionCountByNumber", func(t *testing.T) {

		numberOfTransactions := 5

		// Create the specified number of transactions and wait for them to be mined
		txHashes := transTokenBatch(t, context.Background(), client, uint256.NewInt(encoding.Gwei), testAddress.String(), numberOfTransactions)
		lastTxHash := txHashes[len(txHashes)-1]

		// Get the block information from the last transaction's receipt
		receipt, err := client.RealtimeGetTransactionReceipt(common.HexToHash(lastTxHash))
		require.NoError(t, err)
		require.NotNil(t, receipt, "Transaction receipt should not be nil")

		targetBlockNumber := receipt.BlockNumber.Uint64()
		log.Info(fmt.Sprintf("Testing transaction count for block number: %d", targetBlockNumber))

		// Get transaction count from realtime node
		realtimeTxCount, err := client.RealtimeGetBlockTransactionCountByNumber(targetBlockNumber)
		require.NoError(t, err)

		// Get transaction count from non-realtime node using direct RPC call
		time.Sleep(1 * time.Second)
		blockNumberHex := fmt.Sprintf("0x%x", targetBlockNumber)
		var nonRealtimeTxCountHex string
		err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeTxCountHex, "eth_getBlockTransactionCountByNumber", blockNumberHex)
		require.NoError(t, err)

		// Convert hex string to uint64
		nonRealtimeTxCount, err := strconv.ParseUint(strings.TrimPrefix(nonRealtimeTxCountHex, "0x"), 16, 64)
		require.NoError(t, err)

		require.Equal(t, realtimeTxCount, nonRealtimeTxCount, fmt.Sprintf("Transaction counts should match for block %d: realtime=%d, non-realtime=%d", targetBlockNumber, realtimeTxCount, nonRealtimeTxCount))
	})

	t.Run("getBlockTransactionCountByHash", func(t *testing.T) {

		numberOfTransactions := 5

		// Create the specified number of transactions and wait for them to be mined
		txHashes := transTokenBatch(t, context.Background(), client, uint256.NewInt(encoding.Gwei), testAddress.String(), numberOfTransactions)
		lastTxHash := txHashes[len(txHashes)-1]

		// Get the block information from the last transaction's receipt
		receipt, err := client.RealtimeGetTransactionReceipt(common.HexToHash(lastTxHash))
		require.NoError(t, err)
		require.NotNil(t, receipt, "Transaction receipt should not be nil")

		targetBlockNumber := receipt.BlockNumber.Uint64()
		targetBlockHash := receipt.BlockHash

		log.Info(fmt.Sprintf("Testing transaction count for block %d by hash: %s", targetBlockNumber, targetBlockHash.Hex()))

		// Get the actual transaction count for this block by number
		actualTxCount, err := client.RealtimeGetBlockTransactionCountByNumber(targetBlockNumber)
		require.NoError(t, err)

		// Get transaction count from realtime node by hash
		realtimeTxCount, err := client.RealtimeGetBlockTransactionCountByHash(targetBlockHash)
		require.NoError(t, err)

		// Get transaction count from non-realtime node by hash using direct RPC call
		time.Sleep(1 * time.Second)
		var nonRealtimeTxCountHex string
		err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeTxCountHex, "eth_getBlockTransactionCountByHash", targetBlockHash)
		require.NoError(t, err)

		// Convert hex string to uint64
		nonRealtimeTxCount, err := strconv.ParseUint(strings.TrimPrefix(nonRealtimeTxCountHex, "0x"), 16, 64)
		require.NoError(t, err)

		require.Equal(t, actualTxCount, realtimeTxCount, fmt.Sprintf("Transaction count by hash should match count by number (%d)", actualTxCount))
		require.Equal(t, realtimeTxCount, nonRealtimeTxCount, fmt.Sprintf("Transaction counts should match for block %d hash %s: realtime=%d, non-realtime=%d", targetBlockNumber, targetBlockHash.Hex(), realtimeTxCount, nonRealtimeTxCount))

	})

	// Test getBlockInternalTransactions
	t.Run("getBlockInternalTransactions", func(t *testing.T) {
		txHash := transToken(t, context.Background(), client, uint256.NewInt(encoding.Gwei), testAddress.String())

		// Get the block number directly from the transaction receipt
		receipt, err := client.RealtimeGetTransactionReceipt(common.HexToHash(txHash))
		require.NoError(t, err)
		require.NotNil(t, receipt, "Transaction receipt should not be nil")

		targetBlockNumber := receipt.BlockNumber.Uint64()
		log.Info(fmt.Sprintf("Testing internal transactions for block number: %d", targetBlockNumber))

		// Get internal transactions from realtime node
		realtimeInternalTxs, err := client.RealtimeGetBlockInternalTransactions(targetBlockNumber)
		require.NoError(t, err)
		require.NotNil(t, realtimeInternalTxs, "Realtime internal transactions map should not be nil")

		// Get internal transactions from non-realtime node using direct RPC call
		time.Sleep(1 * time.Second)
		blockNumberHex := fmt.Sprintf("0x%x", targetBlockNumber)
		var nonRealtimeInternalTxs map[common.Hash][]*zktypes.InnerTx
		err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeInternalTxs, "eth_getBlockInternalTransactions", blockNumberHex)
		require.NoError(t, err)
		require.NotNil(t, nonRealtimeInternalTxs, "Non-realtime internal transactions should not be nil")

		require.Equal(t, realtimeInternalTxs, nonRealtimeInternalTxs, fmt.Sprintf("Internal transactions should be identical for block %d", targetBlockNumber))
	})

	t.Run("getTransactionByHash", func(t *testing.T) {
		realtimeTransaction, err := client.RealtimeGetTransactionByHash(txHashCommon, nil)
		require.NoError(t, err)

		// Make direct RPC call to non-realtime node to get JSON response
		var nonRealtimeTransaction rpcTypes.Transaction
		err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeTransaction, "eth_getTransactionByHash", txHashCommon)
		require.NoError(t, err)

		require.Equal(t, realtimeTransaction, nonRealtimeTransaction, fmt.Sprintf("Transactions should be identical for hash %s", txHash))
	})

	t.Run("getRawTransactionByHash", func(t *testing.T) {
		realtimeTransactionBytes, err := client.RealtimeGetRawTransactionByHash(txHashCommon)
		require.NoError(t, err)

		// Make direct RPC call to non-realtime node to get raw transaction hex string
		var nonRealtimeTransaction string
		err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeTransaction, "eth_getRawTransactionByHash", txHashCommon)
		require.NoError(t, err)
		require.NotEmpty(t, nonRealtimeTransaction, "Non-realtime transaction should not be empty")

		// Convert realtime bytes to hex string for comparison
		realtimeTransactionHex := "0x" + hex.EncodeToString(realtimeTransactionBytes)

		require.Equal(t, realtimeTransactionHex, nonRealtimeTransaction, fmt.Sprintf("Raw transactions should be identical for hash %s", txHash))
	})

	t.Run("getTransactionReceipt", func(t *testing.T) {
		realtimeReceipt, err := client.RealtimeGetTransactionReceipt(txHashCommon)
		require.NoError(t, err)

		// Make direct RPC call to non-realtime node to get JSON response
		var nonRealtimeReceipt *types.Receipt
		err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeReceipt, "eth_getTransactionReceipt", txHashCommon)
		require.NoError(t, err)
		require.NotNil(t, nonRealtimeReceipt, "Non-realtime receipt should not be nil")

		require.Equal(t, realtimeReceipt, nonRealtimeReceipt, fmt.Sprintf("Transaction receipts should be identical for hash %s", txHash))
	})

	t.Run("getInternalTransactions", func(t *testing.T) {
		realtimeInternalTxs, err := client.RealtimeGetInternalTransactions(txHashCommon)
		require.NoError(t, err)

		// Make direct RPC call to non-realtime node to get JSON response
		var nonRealtimeInternalTxs []zktypes.InnerTx
		err = nonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeInternalTxs, "eth_getInternalTransactions", txHashCommon)
		require.NoError(t, err)
		require.NotNil(t, nonRealtimeInternalTxs, "Non-realtime internal transactions should not be nil")

		require.Equal(t, realtimeInternalTxs, nonRealtimeInternalTxs, fmt.Sprintf("Internal transactions should be identical for hash %s", txHash))
	})

}
