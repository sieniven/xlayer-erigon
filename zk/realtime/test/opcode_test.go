package test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/accounts/abi/bind"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/test/operations"
	"github.com/stretchr/testify/require"
)

func TestIterativeCreate2AndDestroy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	client, err := ethclient.Dial(DefaultL2NetworkURL)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(DefaultL2AdminPrivateKey[2:])
	require.NoError(t, err)

	// Deploy Factory contract
	factoryAddr := DeployFactoryContract(t, ctx, client)

	salt := big.NewInt(42) // Use a fixed salt for deterministic address
	for i := 0; i < 5; i++ {
		// Deploy initial destroy contract
		SendDeployDestroyContractTx(t, ctx, client, privateKey, factoryAddr, salt)

		destroyAddr := GetContractAddress(t, ctx, client, factoryAddr, salt)
		code, err := RealtimeGetCode(destroyAddr)
		require.NoError(t, err)
		require.NotEmpty(t, code, "Destroy contract code should exist after deploy")

		SendDestroyContractTx(t, ctx, client, privateKey, destroyAddr)

		code, err = RealtimeGetCode(destroyAddr)
		require.NoError(t, err)
		require.Equal(t, code, "0x", "Destroy contract code should not exist after destroy")
	}
}

func DeployFactoryContract(t *testing.T, ctx context.Context, client *ethclient.Client) common.Address {
	// Deploy Factory contract
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.PendingNonceAt(ctx, auth.From)
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

func SendDeployDestroyContractTx(t *testing.T, ctx context.Context, client *ethclient.Client, privateKey *ecdsa.PrivateKey, factoryAddr common.Address, salt *big.Int) {
	destroyBytecode, err := hex.DecodeString(destroyBytecodeStr)
	require.NoError(t, err)
	data, err := factoryABI.Pack("deploy", destroyBytecode, salt)
	require.NoError(t, err)

	nonce, err := client.PendingNonceAt(ctx, common.HexToAddress(DefaultL2AdminAddress))
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

func SendDestroyContractTx(t *testing.T, ctx context.Context, client *ethclient.Client, privateKey *ecdsa.PrivateKey, destroyAddr common.Address) {
	data, err := destroyABI.Pack("destroy", common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)

	nonce, err := client.PendingNonceAt(ctx, common.HexToAddress(DefaultL2AdminAddress))
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

func GetContractAddress(t *testing.T, ctx context.Context, client *ethclient.Client, factoryAddr common.Address, salt *big.Int) common.Address {
	destroyBytecode, err := hex.DecodeString(destroyBytecodeStr)
	require.NoError(t, err)

	// Pack the call to computeAddress
	computeInput, err := factoryABI.Pack("computeAddress", destroyBytecode, salt)
	require.NoError(t, err)
	result, err := RealtimeCall(common.HexToAddress(DefaultL2AdminAddress), factoryAddr, "0x300000", "0x1", "0x0", fmt.Sprintf("0x%x", computeInput))
	require.NoError(t, err)

	// Unpack the result
	return common.HexToAddress(result)
}
