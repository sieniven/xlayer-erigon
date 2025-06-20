package test

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/holiman/uint256"
	ethereum "github.com/ledgerwatch/erigon"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/rpc"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/realtime/kafka/types"
	types "github.com/ledgerwatch/erigon/zk/rpcdaemon"
	"github.com/ledgerwatch/erigon/zkevm/encoding"
	"github.com/ledgerwatch/erigon/zkevm/log"
	logger "github.com/ledgerwatch/log/v3"
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

func TestRealtimeBenchmarkTransactionSubscription(t *testing.T) {
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
	logger := logger.New()
	client, err := ethclient.Dial(DefaultL2NetworkURL)
	require.NoError(t, err)
	wsClient, err := rpc.Dial(DefaultL2NetworkWSURL, logger)
	require.NoError(t, err)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Benchmark variables
	var totalsubDuration time.Duration

	txCh := make(chan kafkaTypes.TransactionMessage)
	sub, err := wsClient.Subscribe(ctx, "realtime", txCh, "realtimeTransactions", true)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Benchmark subscibe realtime transaction
	for i := 0; i < 100; i++ {
		// Send tx
		nativeTransferTx(t, ctx, client, uint256.NewInt(encoding.Gwei), testAddress.String())

		g, _ := errgroup.WithContext(ctx)
		var subDuration time.Duration

		// realtime subscription
		g.Go(func() error {
			startTime := time.Now()

			select {
			case <-txCh:
				subDuration = time.Since(startTime)
				return nil
			case err := <-sub.Err():
				return err
			case <-time.After(DefaultTimeoutTxToBeMined):
				return fmt.Errorf("realtime subscription timeout")
			}
		})

		// Wait for all goroutines to complete
		err = g.Wait()
		require.NoError(t, err)

		totalsubDuration += subDuration

		fmt.Printf("Iteration %v:\n", i)
		fmt.Printf("Realtime transaction subscription duration: %s\n", subDuration)
	}

	avgsubDuration := totalsubDuration / 100

	// Log out metrics
	fmt.Printf("Avg realtime transaction subscription duration: %s\n", avgsubDuration)
}

func TestRealtimeBenchmarkLogSubscription(t *testing.T) {
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
	logger := logger.New()
	client, err := ethclient.Dial(DefaultL2NetworkURL)
	require.NoError(t, err)
	realtimeWSClient, err := rpc.Dial(DefaultL2NetworkWSURL, logger)
	require.NoError(t, err)
	ethWSClient, err := rpc.Dial(DefaultL2NetworkWSURL, logger)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(tmpSenderPrivateKey)
	require.NoError(t, err)
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	senderAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	log.Infof("Sender: %s", senderAddress)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Deploy the contract
	erc20Address := deployERC20Contract(t, ctx, privateKey, client)
	transferAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // Adjust for token decimals (18 in this case)

	startNonce, err := client.PendingNonceAt(context.Background(), senderAddress)
	require.NoError(t, err)

	// Benchmark variables
	var totalRealtimeDuration, totalEthDuration time.Duration

	// Subscirbed topic
	erc20TransferTopic := common.HexToHash(erc20TransferTopicHex)
	q, err := toLogFilterArg(ethereum.FilterQuery{Topics: [][]common.Hash{{erc20TransferTopic}}})
	require.NoError(t, err)

	realtimeCh := make(chan *types.Log)
	realtimeSub, err := realtimeWSClient.Subscribe(ctx, "realtime", realtimeCh, "logs", q)
	require.NoError(t, err)
	defer realtimeSub.Unsubscribe()

	ethCh := make(chan *types.Log)
	ethSub, err := ethWSClient.Subscribe(ctx, "eth", ethCh, "logs", q)
	require.NoError(t, err)
	defer ethSub.Unsubscribe()

	// Benchmark subscibe realtime log
	for i := 0; i < 100; i++ {
		// Send tx
		erc20TransferTx(t, ctx, privateKey, client, transferAmount, testAddress, erc20Address, startNonce+uint64(i))

		g, _ := errgroup.WithContext(ctx)
		var realtimeDuration, ethDuration time.Duration

		// realtime subscription
		g.Go(func() error {
			startTime := time.Now()

			select {
			case log := <-realtimeCh:
				if log.Topics[0] != erc20TransferTopic {
					return fmt.Errorf("realtime subscription fetched unknown logs")
				}
				realtimeDuration = time.Since(startTime)
				return nil
			case err := <-realtimeSub.Err():
				return err
			case <-time.After(DefaultTimeoutTxToBeMined):
				return fmt.Errorf("realtime subscription timeout")
			}
		})

		// eth subscription
		g.Go(func() error {
			startTime := time.Now()

			select {
			case log := <-ethCh:
				if log.Topics[0] != erc20TransferTopic {
					return fmt.Errorf("eth subscription fetched unknown logs")
				}
				ethDuration = time.Since(startTime)
				return nil
			case err := <-ethSub.Err():
				return err
			case <-time.After(DefaultTimeoutTxToBeMined):
				return fmt.Errorf("eth subscription timeout")
			}
		})

		// Wait for all goroutines to complete
		err = g.Wait()
		require.NoError(t, err)

		totalRealtimeDuration += realtimeDuration
		totalEthDuration += ethDuration

		fmt.Printf("Iteration %v:\n", i)
		fmt.Printf("Realtime log subscription duration: %s\n", totalRealtimeDuration)
		fmt.Printf("Eth log subscription duration: %s\n", totalEthDuration)
	}

	avgRealtimeDuration := totalRealtimeDuration / 100
	avgEthDuration := totalEthDuration / 100

	// Log out metrics
	fmt.Printf("Avg realtime log subscription duration: %s\n", avgRealtimeDuration)
	fmt.Printf("Avg eth log subscription duration: %s\n", avgEthDuration)
}
