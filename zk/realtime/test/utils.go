package test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holiman/uint256"
	ethereum "github.com/ledgerwatch/erigon"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/accounts/abi"
	"github.com/ledgerwatch/erigon/accounts/abi/bind"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/test/operations"
	"github.com/stretchr/testify/require"
)

var (
	erc20ABI, _ = abi.JSON(strings.NewReader(erc20ABIJson))
)

// setupRealtimeTestEnvironment creates a test environment with necessary data for tests
func setupRealtimeTestEnvironment(t *testing.T) uint64 {
	// Wait for at least one block to be available
	var blockNumber uint64
	var err error
	for i := 0; i < 30; i++ {
		blockNumber, err = RealtimeBlockNumber()
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

func nativeTransferTx(t *testing.T, ctx context.Context, client *ethclient.Client, amount *uint256.Int, toAddress string) types.Transaction {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  auth.From,
		To:    &to,
		Value: amount,
	})
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
	client *ethclient.Client,
) common.Address {
	publicKey := privateKey.Public()
	tmpPublicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	fromAddress := crypto.PubkeyToAddress(*tmpPublicKeyECDSA)
	fmt.Printf("Sender: %s\n", fromAddress)

	nativeTransferTx(t, ctx, client,
		new(uint256.Int).Mul(uint256.NewInt(1000), uint256.NewInt(1e18)),
		fromAddress.String())

	nonce, err := client.PendingNonceAt(ctx, fromAddress)
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

	fmt.Printf("ERC20 Contract deployed at: %s, transaction hash: %s\n", erc20Address.Hex(), tx.Hash().Hex())
	// Wait for contract deployment to be mined
	bind.WaitDeployed(ctx, client, tx)

	return erc20Address
}

func erc20TransferTx(
	t *testing.T,
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	client *ethclient.Client,
	amount *big.Int,
	fromAddress common.Address,
	toAddress common.Address,
	erc20Address common.Address,
	nonce uint64,
) types.Transaction {

	// Prepare transfer data
	data, err := erc20ABI.Pack("transfer", toAddress, amount)
	require.NoError(t, err)

	transferERC20TokenTx := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &erc20Address,
			Gas:   60000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(100),
	}

	signer := types.MakeSigner(operations.GetTestChainConfig(DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(transferERC20TokenTx, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	return signedTx
}

// RevertReason returns the revert reason for a tx that has a receipt with failed status
func RevertReason(
	ctx context.Context,
	c ethClienter,
	tx types.Transaction,
	blockNumber *big.Int,
) (string, error) {
	if tx == nil {
		return "", nil
	}

	from, _ := tx.GetSender()
	msg := ethereum.CallMsg{
		From: from,
		To:   tx.GetTo(),
		Gas:  tx.GetGas(),

		Value: tx.GetValue(),
		Data:  tx.GetData(),
	}
	hex, err := c.CallContract(ctx, msg, blockNumber)
	if err != nil {
		return "", err
	}

	unpackedMsg, err := abi.UnpackRevert(hex)
	if err != nil {
		fmt.Printf("failed to get the revert message for tx %v: %v\n", tx.Hash(), err)
		return "", errors.New("execution reverted")
	}

	return unpackedMsg, nil
}

func transHexToUint64(hex json.RawMessage) (uint64, error) {
	var result string
	err := json.Unmarshal(hex, &result)
	if err != nil {
		return 0, err
	}

	if len(result) > 1 && (result[:2] == "0x" || result[:2] == "0X") {
		result = result[2:]
	}

	result1, err := strconv.ParseUint(result, 16, 64)
	if err != nil {
		return 0, err
	}

	return result1, nil
}
