package test

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/zkevm/encoding"
	"github.com/ledgerwatch/erigon/zkevm/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestRealtimeBenchmarkNativeTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	blockNumber := setupRealtimeTestEnvironment(t)

	t.Run("RealtimeGetBlockTransactionCountByNumber", func(t *testing.T) {
		transactionCount, err := RealtimeGetBlockTransactionCountByNumber(blockNumber)
		require.NoError(t, err)
		log.Infof("RealtimeGetBlockTransactionCountByNumber result: %d", transactionCount)
	})

	ctx := context.Background()
	client, err := ethclient.Dial(DefaultL2NetworkURL)
	require.NoError(t, err)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Benchmark variables
	var totalRealtimeDuration, totalEthDuration time.Duration
	var totalRealtimeBalanceDuration, totalEthBalanceDuration time.Duration

	// Benchmark transfer tx to test address
	for i := 0; i < 100; i++ {
		balance, err := EthGetBalance(testAddress, "latest")
		require.NoError(t, err)
		realtimeBalance, err := RealtimeGetBalance(testAddress)
		require.NoError(t, err)
		require.Equal(t, balance.String(), realtimeBalance.String())

		// Send tx
		signedTx := nativeTransferTx(t, context.Background(), client, uint256.NewInt(encoding.Gwei), testAddress.String())

		// Run stateless benchmark
		g, ctx := errgroup.WithContext(ctx)
		var realtimeDuration, ethDuration time.Duration
		g.Go(func() error {
			duration, err := WaitCallback(ctx, client, signedTx, common.Address{}, *signedTx.GetTo(), balance, DefaultTimeoutTxToBeMined, WaitMinedRealtime)
			if err != nil {
				return err
			}
			realtimeDuration = duration
			return nil
		})

		g.Go(func() error {
			duration, err := WaitCallback(ctx, client, signedTx, common.Address{}, *signedTx.GetTo(), balance, DefaultTimeoutTxToBeMined, WaitMinedEth)
			if err != nil {
				return err
			}
			ethDuration = duration
			return nil
		})

		// Run state benchmark
		var realtimeBalanceDuration, ethBalanceDuration time.Duration
		g.Go(func() error {
			duration, err := WaitCallback(ctx, client, signedTx, common.Address{}, *signedTx.GetTo(), balance, DefaultTimeoutTxToBeMined, WaitAccountBalanceRealtime)
			if err != nil {
				return err
			}
			realtimeBalanceDuration = duration
			return nil
		})

		g.Go(func() error {
			duration, err := WaitCallback(ctx, client, signedTx, common.Address{}, *signedTx.GetTo(), balance, DefaultTimeoutTxToBeMined, WaitAccountBalanceEth)
			if err != nil {
				return err
			}
			ethBalanceDuration = duration
			return nil
		})

		// Wait for all goroutines to complete
		err = g.Wait()
		require.NoError(t, err)

		totalRealtimeDuration += realtimeDuration
		totalEthDuration += ethDuration
		totalRealtimeBalanceDuration += realtimeBalanceDuration
		totalEthBalanceDuration += ethBalanceDuration

		fmt.Printf("Iteration %v:\n", i)
		fmt.Printf("Realtime stateless duration: %s\n", realtimeDuration)
		fmt.Printf("Eth stateless duration: %s\n", ethDuration)
		fmt.Printf("Realtime state duration: %s\n", realtimeBalanceDuration)
		fmt.Printf("Eth state duration: %s\n", ethBalanceDuration)
	}

	avgRealtimeDuration := totalRealtimeDuration / 100
	avgEthDuration := totalEthDuration / 100
	avgRealtimeBalanceDuration := totalRealtimeBalanceDuration / 100
	avgEthBalanceDuration := totalEthBalanceDuration / 100

	// Log out metrics
	fmt.Printf("Avg realtime stateless native tx transfer confirmation duration: %s\n", avgRealtimeDuration)
	fmt.Printf("Avg eth stateless native tx transfer confirmation duration: %s\n", avgEthDuration)
	fmt.Printf("Avg realtime state native tx transfer confirmation duration: %s\n", avgRealtimeBalanceDuration)
	fmt.Printf("Avg eth state native tx transfer confirmation duration: %s\n", avgEthBalanceDuration)
}

func TestRealtimeBenchmarkERC20Transfer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	blockNumber := setupRealtimeTestEnvironment(t)

	t.Run("RealtimeGetBlockTransactionCountByNumber", func(t *testing.T) {
		transactionCount, err := RealtimeGetBlockTransactionCountByNumber(blockNumber)
		require.NoError(t, err)
		log.Infof("RealtimeGetBlockTransactionCountByNumber result: %d", transactionCount)
	})

	ctx := context.Background()
	client, err := ethclient.Dial(DefaultL2NetworkURL)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(tmpSenderPrivateKey)
	require.NoError(t, err)
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	senderAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	log.Infof("Sender: %s", senderAddress)

	// Default test address for tests that require an address
	fromAddress := common.HexToAddress(DefaultL2AdminAddress)
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Deploy the contract
	erc20Address := deployERC20Contract(t, ctx, privateKey, client)
	transferAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // Adjust for token decimals (18 in this case)

	startNonce, err := client.PendingNonceAt(context.Background(), senderAddress)
	require.NoError(t, err)

	// Benchmark variables
	var totalRealtimeDuration, totalEthDuration time.Duration
	var totalRealtimeBalanceDuration, totalEthBalanceDuration time.Duration

	// Benchmark erc20 transfer tx
	for i := 0; i < 100; i++ {
		balance, err := EthGetTokenBalance(ctx, client, testAddress, erc20Address)
		require.NoError(t, err)
		realtimeBalance, err := RealtimeGetTokenBalance(ctx, client, fromAddress, testAddress, erc20Address)
		require.NoError(t, err)
		require.Equal(t, balance.String(), realtimeBalance.String())

		signedTx := erc20TransferTx(t, ctx, privateKey, client, transferAmount, testAddress, erc20Address, startNonce+uint64(i))

		// Run stateless benchmark
		g, ctx := errgroup.WithContext(ctx)
		var realtimeDuration, ethDuration time.Duration
		g.Go(func() error {
			duration, err := WaitCallback(ctx, client, signedTx, common.Address{}, *signedTx.GetTo(), balance, DefaultTimeoutTxToBeMined, WaitMinedRealtime)
			if err != nil {
				return err
			}
			realtimeDuration = duration
			return nil
		})

		g.Go(func() error {
			duration, err := WaitCallback(ctx, client, signedTx, common.Address{}, *signedTx.GetTo(), balance, DefaultTimeoutTxToBeMined, WaitMinedEth)
			if err != nil {
				return err
			}
			ethDuration = duration
			return nil
		})

		// Run state benchmark
		var realtimeBalanceDuration, ethBalanceDuration time.Duration
		g.Go(func() error {
			duration, err := WaitCallback(ctx, client, signedTx, fromAddress, testAddress, balance, DefaultTimeoutTxToBeMined, WaitTokenBalanceRealtime)
			if err != nil {
				return err
			}
			realtimeBalanceDuration = duration
			return nil
		})

		g.Go(func() error {
			duration, err := WaitCallback(ctx, client, signedTx, fromAddress, testAddress, balance, DefaultTimeoutTxToBeMined, WaitTokenBalanceEth)
			if err != nil {
				return err
			}
			ethBalanceDuration = duration
			return nil
		})

		// Wait for all goroutines to complete
		err = g.Wait()
		require.NoError(t, err)

		totalRealtimeDuration += realtimeDuration
		totalEthDuration += ethDuration
		totalRealtimeBalanceDuration += realtimeBalanceDuration
		totalEthBalanceDuration += ethBalanceDuration

		fmt.Printf("Iteration %v:\n", i)
		fmt.Printf("Realtime stateless duration: %s\n", realtimeDuration)
		fmt.Printf("Eth stateless duration: %s\n", ethDuration)
		fmt.Printf("Realtime state duration: %s\n", realtimeBalanceDuration)
		fmt.Printf("Eth state duration: %s\n", ethBalanceDuration)
	}

	avgRealtimeDuration := totalRealtimeDuration / 100
	avgEthDuration := totalEthDuration / 100
	avgRealtimeBalanceDuration := totalRealtimeBalanceDuration / 100
	avgEthBalanceDuration := totalEthBalanceDuration / 100

	// Log out metrics
	fmt.Printf("Avg realtime stateless erc20 tx transfer confirmation duration: %s\n", avgRealtimeDuration)
	fmt.Printf("Avg eth stateless erc20 tx transfer confirmation duration: %s\n", avgEthDuration)
	fmt.Printf("Avg realtime state erc20 tx transfer confirmation duration: %s\n", avgRealtimeBalanceDuration)
	fmt.Printf("Avg eth state erc20 tx transfer confirmation duration: %s\n", avgEthBalanceDuration)
}
