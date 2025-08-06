package test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/accounts/abi"
	"github.com/ledgerwatch/erigon/accounts/abi/bind"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/test/operations"
	"github.com/ledgerwatch/erigon/zk/realtime/rtclient"
	"github.com/ledgerwatch/erigon/zkevm/log"
	"github.com/stretchr/testify/require"
)

var (
	erc20ABI, _         = abi.JSON(strings.NewReader(erc20ABIJson))
	factoryABI, _       = abi.JSON(strings.NewReader(factoryABIJson))
	destroyABI, _       = abi.JSON(strings.NewReader(destroyABIJson))
	createDestroyABI, _ = abi.JSON(strings.NewReader(createDestroyABIJson))
)

// setupRealtimeTestEnvironment creates a test environment with necessary data for tests
func setupRealtimeTestEnvironment(t *testing.T, client *rtclient.RealtimeClient) uint64 {
	// Wait for at least one block to be available
	var blockNumber uint64
	var err error
	for i := 0; i < 30; i++ {
		blockNumber, err = client.RealtimeBlockNumber()
		require.NoError(t, err)
		fmt.Printf("Realtime block number: %d, attempt: %v\n", blockNumber, i)
		if blockNumber > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.Greater(t, blockNumber, uint64(0), "Realtime block number should be greater than 0")

	// Get a block hash to use for tests
	batchNum, err := operations.GetBatchNumber()
	require.NoError(t, err)

	batch, err := operations.GetBatchByNumber(new(big.Int).SetUint64(batchNum))
	require.NoError(t, err)
	require.NotEmpty(t, batch.Blocks, "Batch should contain at least one block")

	return blockNumber
}

func nativeTransferTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, amount *uint256.Int, toAddress string) types.Transaction {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gas := uint64(21000)
	require.NoError(t, err)

	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: amount,
		},
		GasPrice: uint256.MustFromBig(gasPrice),
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	return signedTx
}

func deployERC20Contract(
	t *testing.T,
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	client *rtclient.RealtimeClient,
) common.Address {
	publicKey := privateKey.Public()
	tmpPublicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	fromAddress := crypto.PubkeyToAddress(*tmpPublicKeyECDSA)
	fmt.Printf("Sender: %s\n", fromAddress)

	nonce, err := client.RealtimeGetTransactionCount(fromAddress)
	require.NoError(t, err)

	// Define gas parameters
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	// Set up transaction options
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(195))
	require.NoError(t, err)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	// Deploy the contract
	erc20Bytecode, err := hex.DecodeString(erc20BytecodeStr)
	require.NoError(t, err)
	erc20Address, tx, _, err := bind.DeployContract(auth, erc20ABI, erc20Bytecode, client)
	require.NoError(t, err)

	// Wait for contract deployment to be mined
	err = WaitTxToBeMined(ctx, client, tx, DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
	fmt.Printf("ERC20 Contract deployed at: %s, transaction hash: %s\n", erc20Address.Hex(), tx.Hash().Hex())

	return erc20Address
}

func erc20TransferTx(
	t *testing.T,
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	client *rtclient.RealtimeClient,
	amount *big.Int,
	toAddress common.Address,
	erc20Address common.Address,
	nonce uint64,
) types.Transaction {

	// Prepare transfer data
	data, err := erc20ABI.Pack("transfer", toAddress, amount)
	require.NoError(t, err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	transferERC20TokenTx := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &erc20Address,
			Gas:   60000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.MustFromBig(gasPrice),
	}

	signer := types.MakeSigner(operations.GetTestChainConfig(DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(transferERC20TokenTx, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	return signedTx
}

func transToken(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, amount *uint256.Int, toAddress string) string {
	return transTokenWithFrom(t, ctx, client, operations.DefaultL2AdminPrivateKey, amount, toAddress)
}

// Creates multiple transactions in a batch and waits for them all to be mined
// If fromPrivateKey is empty, uses the default admin private key
func transTokenBatch(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, amount *uint256.Int, toAddress string, batchSize int, fromPrivateKey ...string) []string {
	privateKey := operations.DefaultL2AdminPrivateKey
	if len(fromPrivateKey) > 0 && fromPrivateKey[0] != "" {
		privateKey = fromPrivateKey[0]
	}
	var txHashes []string
	var transactions []types.Transaction

	// Create all transactions first
	for i := 0; i < batchSize; i++ {
		chainID, err := client.ChainID(ctx)
		require.NoError(t, err)
		auth, err := operations.GetAuth(privateKey, chainID.Uint64())
		require.NoError(t, err)
		nonce, err := client.RealtimeGetTransactionCount(auth.From)
		require.NoError(t, err)
		gasPrice, err := client.SuggestGasPrice(ctx)
		require.NoError(t, err)

		to := common.HexToAddress(toAddress)
		gas := uint64(21000)

		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   gas,
				Value: amount,
			},
			GasPrice: uint256.MustFromBig(gasPrice),
		}

		privKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
		require.NoError(t, err)

		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privKey)
		require.NoError(t, err)

		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)

		txHashes = append(txHashes, signedTx.Hash().String())
		transactions = append(transactions, signedTx)
	}

	// Wait for all transactions to be mined
	for _, tx := range transactions {
		err := WaitTxToBeMined(ctx, client, tx, DefaultTimeoutTxToBeMined)
		require.NoError(t, err)
	}

	log.Info(fmt.Sprintf("All %d transactions have been mined successfully", len(transactions)))
	return txHashes
}

func transTokenWithFrom(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, fromPrivateKey string, amount *uint256.Int, toAddress string) string {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(fromPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gas := uint64(21000)
	require.NoError(t, err)
	log.Info(fmt.Sprintf("gas: %d", gas))
	log.Info(fmt.Sprintf("gasPrice: %d", gasPrice))

	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: amount,
		},
		GasPrice: uint256.MustFromBig(gasPrice),
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(fromPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	return signedTx.Hash().String()
}

func DeployFactoryContract(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient) common.Address {
	// Deploy Factory contract
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	factoryBytecode, err := hex.DecodeString(factoryBytecodeStr)
	require.NoError(t, err)
	factoryAddr, tx, _, err := bind.DeployContract(auth, factoryABI, factoryBytecode, client)
	require.NoError(t, err)

	fmt.Printf("Factory contract deployed at: %s, transaction hash: %s\n", factoryAddr.Hex(), tx.Hash().Hex())
	bind.WaitDeployed(ctx, client, tx)

	return factoryAddr
}

func SendDeployDestroyContractTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, privateKey *ecdsa.PrivateKey, factoryAddr common.Address, salt *big.Int) {
	destroyBytecode, err := hex.DecodeString(destroyBytecodeStr)
	require.NoError(t, err)
	data, err := factoryABI.Pack("deploy", destroyBytecode, salt)
	require.NoError(t, err)

	nonce, err := client.RealtimeGetTransactionCount(common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	deployTx := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &factoryAddr,
			Gas:   3000000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(uint64(gasPrice.Uint64())),
	}

	signer := types.MakeSigner(operations.GetTestChainConfig(DefaultL2ChainID), 1, 0)
	require.NoError(t, err)
	signedTx, err := types.SignTx(deployTx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
}

func SendDestroyContractTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, privateKey *ecdsa.PrivateKey, destroyAddr common.Address) {
	data, err := destroyABI.Pack("destroy", common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)

	nonce, err := client.RealtimeGetTransactionCount(common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	destroyTx := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &destroyAddr,
			Gas:   3000000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(uint64(gasPrice.Uint64())),
	}

	signer := types.MakeSigner(operations.GetTestChainConfig(DefaultL2ChainID), 1, 0)
	require.NoError(t, err)
	signedTx, err := types.SignTx(destroyTx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
}

func GetContractAddressFromFactory(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, factoryAddr common.Address, salt *big.Int) common.Address {
	destroyBytecode, err := hex.DecodeString(destroyBytecodeStr)
	require.NoError(t, err)

	// Pack the call to computeAddress
	computeInput, err := factoryABI.Pack("computeAddress", destroyBytecode, salt)
	require.NoError(t, err)
	result, err := client.RealtimeCall(common.HexToAddress(DefaultL2AdminAddress), factoryAddr, "0x300000", "0x1", "0x0", fmt.Sprintf("0x%x", computeInput))
	require.NoError(t, err)

	// Unpack the result
	return common.HexToAddress(result)
}

func DeployCreateDestroyContract(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient) common.Address {
	// Deploy Factory contract
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	createDestroyBytecode, err := hex.DecodeString(createDestroyBytecodeStr)
	require.NoError(t, err)
	createDestroyAddr, tx, _, err := bind.DeployContract(auth, createDestroyABI, createDestroyBytecode, client)
	require.NoError(t, err)

	fmt.Printf("Create Destroy contract deployed at: %s, transaction hash: %s\n", createDestroyAddr.Hex(), tx.Hash().Hex())
	bind.WaitDeployed(ctx, client, tx)

	return createDestroyAddr
}

func SendCreateAndDestroyTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, privateKey *ecdsa.PrivateKey, createDestroyAddr common.Address, salt *big.Int) {
	data, err := createDestroyABI.Pack("createAndDestroy", salt, common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)

	nonce, err := client.RealtimeGetTransactionCount(common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	createAndDestroyTx := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &createDestroyAddr,
			Gas:   3000000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(uint64(gasPrice.Uint64())),
	}

	signer := types.MakeSigner(operations.GetTestChainConfig(DefaultL2ChainID), 1, 0)
	require.NoError(t, err)
	signedTx, err := types.SignTx(createAndDestroyTx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
}

func GetContractAddressFromCreateDestroy(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, createDestroyAddr common.Address, salt *big.Int) common.Address {
	destroyBytecode, err := hex.DecodeString(destroyBytecodeStr)
	require.NoError(t, err)

	// Pack the call to computeAddress
	computeInput, err := factoryABI.Pack("computeAddress", destroyBytecode, salt)
	require.NoError(t, err)
	result, err := client.RealtimeCall(common.HexToAddress(DefaultL2AdminAddress), createDestroyAddr, "0x300000", "0x1", "0x0", fmt.Sprintf("0x%x", computeInput))
	require.NoError(t, err)

	// Unpack the result
	return common.HexToAddress(result)
}

// RevertReasonRealtime returns the revert reason for a tx that has a receipt with failed status
func RevertReasonRealtime(
	ctx context.Context,
	client *rtclient.RealtimeClient,
	tx types.Transaction,
) (string, error) {
	if tx == nil {
		return "", nil
	}

	from, _ := tx.GetSender()
	hex, err := client.RealtimeCall(from, *tx.GetTo(), fmt.Sprintf("0x%x", tx.GetGas()), fmt.Sprintf("0x%x", tx.GetPrice()), fmt.Sprintf("0x%x", tx.GetValue()), fmt.Sprintf("0x%x", tx.GetData()))
	if err != nil {
		return "", err
	}

	unpackedMsg, err := abi.UnpackRevert(common.FromHex(hex))
	if err != nil {
		fmt.Printf("failed to get the revert message for tx %v: %v\n", tx.Hash(), err)
		return "", errors.New("execution reverted")
	}

	return unpackedMsg, nil
}

// ConfirmationLevel type used to describe the confirmation level of a transaction
type ConfirmationLevel int

// PoolConfirmationLevel indicates that transaction is added into the pool
const PoolConfirmationLevel ConfirmationLevel = 0

// TrustedConfirmationLevel indicates that transaction is  added into the trusted state
const TrustedConfirmationLevel ConfirmationLevel = 1

// VirtualConfirmationLevel indicates that transaction is  added into the virtual state
const VirtualConfirmationLevel ConfirmationLevel = 2

// VerifiedConfirmationLevel indicates that transaction is  added into the verified state
const VerifiedConfirmationLevel ConfirmationLevel = 3

// ApplyL2Txs sends the given L2 txs, waits for them to be consolidated and
// checks the final state.
func ApplyL2Txs(ctx context.Context, txs []*types.Transaction, auth *bind.TransactOpts, client *rtclient.RealtimeClient, confirmationLevel ConfirmationLevel) ([]*big.Int, error) {
	var err error
	waitToBeMined := confirmationLevel != PoolConfirmationLevel
	sentTxs, err := applyTxs(ctx, txs, auth, client, waitToBeMined)
	if err != nil {
		return nil, err
	}
	if confirmationLevel == PoolConfirmationLevel {
		return nil, nil
	}

	l2BlockNumbers := make([]*big.Int, 0, len(sentTxs))
	for _, txTemp := range sentTxs {
		// check transaction nonce against transaction reported L2 block number
		receipt, err := client.TransactionReceipt(ctx, txTemp.Hash())
		if err != nil {
			return nil, err
		}

		// get L2 block number
		l2BlockNumbers = append(l2BlockNumbers, receipt.BlockNumber)
		if confirmationLevel == TrustedConfirmationLevel {
			continue
		}

		if confirmationLevel == VirtualConfirmationLevel {
			continue
		}
	}

	return l2BlockNumbers, nil
}

func applyTxs(ctx context.Context, txs []*types.Transaction, auth *bind.TransactOpts, client *rtclient.RealtimeClient, waitToBeMined bool) ([]types.Transaction, error) {
	var sentTxs []types.Transaction

	for i := 0; i < len(txs); i++ {
		signedTx, err := auth.Signer(auth.From, *txs[i])
		if err != nil {
			return nil, err
		}
		log.Infof("Sending Tx %v Nonce %v", signedTx.Hash(), signedTx.GetNonce())
		err = client.SendTransaction(context.Background(), signedTx)
		if err != nil {
			return nil, err
		}

		sentTxs = append(sentTxs, signedTx)
	}
	if !waitToBeMined {
		return nil, nil
	}

	// wait for TX to be mined
	timeout := 180 * time.Second //nolint:gomnd
	for _, tx := range sentTxs {
		log.Infof("Waiting Tx %s to be mined", tx.Hash())
		err := WaitTxToBeMined(ctx, client, tx, timeout)
		if err != nil {
			return nil, err
		}
		log.Infof("Tx %s mined successfully", tx.Hash())
	}
	nTxs := len(txs)
	if nTxs > 1 {
		log.Infof("%d transactions added into the trusted state successfully.", nTxs)
	} else {
		log.Info("transaction added into the trusted state successfully.")
	}

	return sentTxs, nil
}
