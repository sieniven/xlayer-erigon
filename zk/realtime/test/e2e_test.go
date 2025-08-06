//go:build !skip_smoke_realtime
// +build !skip_smoke_realtime

package test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
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
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/zk/realtime/rtclient"
	"gopkg.in/yaml.v2"

	"github.com/ledgerwatch/erigon/test/operations"
	"github.com/ledgerwatch/erigon/zkevm/encoding"
	"github.com/ledgerwatch/erigon/zkevm/etherman/smartcontracts/polygonzkevmbridge"
	"github.com/ledgerwatch/erigon/zkevm/log"

	"github.com/stretchr/testify/require"
)

// The realtime e2e test is the e2e/smoke_test.go for xlayer-erigon.
const (
	blockAddress    = "0xdD2FD4581271e230360230F9337D5c0430Bf44C0"
	blockPrivateKey = "0xde9be858da4a475276426320d5e9262ecfc3ba460bfac56360bfa6c4c28b4ee0"

	testVerified                       = true
	tmpSenderPrivateKey                = "363ea277eec54278af051fb574931aec751258450a286edce9e1f64401f3b9c8"
	specificProjectSenderPrivateKey    = "100f4e42de757bdfa31122dfc5bc00f1afe508b7b3c214a96fa00cbf05d979cf"
	nonSpecificProjectSenderPrivateKay = "fc1c22e30c1d9e4f6449b3c44c2991dbc6a202d8895d18fefa224296c01949cd"
	erc20FreeGasAddressStr             = "0xAD1D01007a56EE0A4FFD0488fb58fC6500Cb1fbE"
)

func TestGetBatchSealTime(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// latest batch seal time
	var batchNum uint64
	var batchSealTime uint64
	var err error
	for i := 0; i < 50; i++ {
		batchNum, err = operations.GetBatchNumber()
		require.NoError(t, err)
		batchSealTime, err = operations.GetBatchSealTime(new(big.Int).SetUint64(batchNum))
		require.Equal(t, batchSealTime, uint64(0))
		log.Infof("Batch number: %d, times:%v", batchNum, i)
		if batchNum > 1 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// old batch seal time
	batchNum = batchNum - 1
	batch, err := operations.GetBatchByNumber(new(big.Int).SetUint64(batchNum))
	var maxTime uint64
	for _, block := range batch.Blocks {
		blockInfo, err := operations.GetBlockByHash(common.HexToHash(block.(string)))
		require.NoError(t, err)
		log.Infof("Block Timestamp: %+v", blockInfo.Timestamp)
		blockTime := uint64(blockInfo.Timestamp)
		if blockTime > maxTime {
			maxTime = blockTime
		}
	}
	batchSealTime, err = operations.GetBatchSealTime(new(big.Int).SetUint64(batchNum))
	require.NoError(t, err)
	log.Infof("Max block time: %d, batchSealTime: %d", maxTime, batchSealTime)
	require.Equal(t, maxTime, batchSealTime)
}

func TestBridgeTx(t *testing.T) {
	ctx := context.Background()
	l1Client, err := ethclient.Dial(operations.DefaultL1NetworkURL)
	require.NoError(t, err)
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	l2Client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	transToken(t, ctx, l2Client, uint256.NewInt(encoding.Gwei), operations.DefaultL2AdminAddress)

	amount := new(big.Int).SetUint64(100)
	//layer2 network id
	var destNetwork uint32 = 1
	destAddr := common.HexToAddress(operations.DefaultL2AdminAddress)
	auth, err := operations.GetAuth(operations.DefaultL1AdminPrivateKey, operations.DefaultL1ChainID)
	require.NoError(t, err)

	wethAddress := common.HexToAddress("0x95076baf95000f2e67b2f88998a26d82140308ca")
	wethToken, err := operations.NewToken(wethAddress, l2Client)
	require.NoError(t, err)
	balanceBefore, err := wethToken.BalanceOf(&bind.CallOpts{}, destAddr)
	require.NoError(t, err)
	log.Infof("balanceBefore:%d", balanceBefore)

	err = sendBridgeAsset(ctx, common.Address{}, amount, destNetwork, &destAddr, []byte{}, auth, common.HexToAddress(operations.BridgeAddr), l1Client)
	require.NoError(t, err)

	const maxAttempts = 120

	var balanceAfter *big.Int
	for i := 0; i < maxAttempts; i++ {
		time.Sleep(1 * time.Second)

		balanceAfter, err = wethToken.BalanceOf(&bind.CallOpts{}, destAddr)
		require.NoError(t, err)
		log.Infof("balanceAfter:%d", balanceAfter)

		if balanceAfter.Cmp(balanceBefore) > 0 {
			return
		}
	}

	t.Errorf("bridge transaction failed after %d seconds: balance did not increase (before: %s, after: %s)",
		maxAttempts,
		balanceBefore.String(),
		balanceAfter.String(),
	)
}

func TestClaimTx(t *testing.T) {
	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)
	transToken(t, ctx, client, uint256.NewInt(encoding.Gwei), operations.DefaultL2AdminAddress)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2AdminAddress)
	nonce, err := client.PendingNonceAt(ctx, from)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: uint256.NewInt(10),
	})
	require.NoError(t, err)
	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: uint256.NewInt(10),
		},
		GasPrice: uint256.MustFromBig(big.NewInt(0)),
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
}

func TestNewAccFreeGas(t *testing.T) {
	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)
	transToken(t, ctx, client, uint256.NewInt(encoding.Gwei), operations.DefaultL2AdminAddress)
	var gas uint64 = 21000

	//newAcc transfer failed
	from := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	to := common.HexToAddress(operations.DefaultL2AdminAddress)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: uint256.NewInt(0),
		},
		GasPrice: uint256.MustFromBig(big.NewInt(0)),
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2NewAcc1PrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "RPC error response: FEE_TOO_LOW: underpriced"), "Expected error message not found")
	err = WaitTxToBeMined(ctx, client, signedTx, 5)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "context deadline exceeded"), "Expected error message not found")

	// seq -> newAcc
	from = common.HexToAddress(operations.DefaultL2AdminAddress)
	to = common.HexToAddress(operations.DefaultL2NewAcc1Address)
	nonce, err = client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	tx = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: uint256.NewInt(10),
		},
		GasPrice: uint256.MustFromBig(big.NewInt(0)),
	}
	privateKey, err = crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL1AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signedTx, err = types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
	err = WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	// newAcc transfer success
	from = common.HexToAddress(operations.DefaultL2NewAcc1Address)
	to = common.HexToAddress(operations.DefaultL2AdminAddress)
	nonce, err = client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	tx = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   gas,
			Value: uint256.NewInt(0),
		},
		GasPrice: uint256.MustFromBig(big.NewInt(0)),
	}
	privateKey, err = crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2NewAcc1PrivateKey, "0x"))
	require.NoError(t, err)
	signedTx, err = types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
	err = WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
}
func TestWhiteAndBlockList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	blockAddressConverted := common.HexToAddress(blockAddress)
	nonBlockAddress := common.HexToAddress(operations.DefaultL2NewAcc1Address)

	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &blockAddressConverted,
		Value: uint256.NewInt(10),
	})
	require.NoError(t, err)

	var txToBlockAddress types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &blockAddressConverted,
			Gas:   gas,
			Value: uint256.NewInt(10),
		},
		GasPrice: uint256.MustFromBig(gasPrice),
	}

	var txToNonBlockAddress types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &nonBlockAddress,
			Gas:   gas,
			Value: uint256.NewInt(10),
		},
		GasPrice: uint256.MustFromBig(gasPrice),
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)

	signedTxToBlockAddress, err := types.SignTx(txToBlockAddress, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTxToBlockAddress)
	log.Infof("err:%v", err)
	require.True(t, strings.Contains(err.Error(), "INTERNAL_ERROR: blocked receiver"))

	signedTxToNonBlockAddress, err := types.SignTx(txToNonBlockAddress, *signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTxToNonBlockAddress)
	require.NoError(t, err)

	//TODO: sender in blocklist should fail
	//now only admin account have balance. So we may add another account that has balance.
}

func TestRPCAPI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config, err := LoadConfig("../../test/config/test.erigon.rpc.rt.config.yaml")
	require.NoError(t, err)

	if config.HTTPAPIKeys != "" {

		_, err := operations.GetEthSyncing(DefaultL2NetworkRealtimeURL)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "no authentication"))

		_, err = operations.GetEthSyncing(DefaultL2NetworkRealtimeURL + "/45543e0adc5dd3e316044909d32501a5")
		require.NoError(t, err)
	} else {

		var rateErr error
		for i := 0; i < 1000; i++ {
			_, err1 := operations.GetEthSyncing(DefaultL2NetworkRealtimeURL)
			if err1 != nil {
				rateErr = err1
				break
			}
		}

		require.True(t, strings.Contains(rateErr.Error(), "rate limit exceeded"))
	}
}

func TestChainID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chainID, err := operations.GetNetVersion(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	require.Equal(t, chainID, operations.DefaultL2ChainID)
}

func TestInnerTx(t *testing.T) {
	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)
	txHash := transToken(t, ctx, client, uint256.NewInt(encoding.Gwei), operations.DefaultL2AdminAddress)
	log.Infof("txHash: %s", txHash)

	result, err := operations.GetInternalTransactions(common.HexToHash(txHash))
	require.NoError(t, err)
	require.Greater(t, len(result), 0)
	require.Equal(t, result[0].From, operations.DefaultL2AdminAddress)

	tx, err := operations.GetTransactionByHash(common.HexToHash(txHash))
	require.NoError(t, err)
	log.Infof("tx: %+v", tx.BlockNumber)
	result1, err := operations.GetBlockInternalTransactions(new(big.Int).SetUint64(uint64(*tx.BlockNumber)))
	require.NoError(t, err)
	require.Greater(t, len(result1), 0)
	require.Equal(t, result1[common.HexToHash(txHash)][0].From, operations.DefaultL2AdminAddress)
}

func TestEthTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	if !testVerified {
		return
	}

	ctx := context.Background()
	auth, err := operations.GetAuth(operations.DefaultL2AdminPrivateKey, operations.DefaultL2ChainID)
	require.NoError(t, err)
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   21000,
			Value: uint256.NewInt(0),
		},
		GasPrice: uint256.NewInt(10 * encoding.Gwei),
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	var txs []*types.Transaction
	txs = append(txs, &signedTx)
	_, err = ApplyL2Txs(ctx, txs, auth, client, VerifiedConfirmationLevel)
	require.NoError(t, err)
}

func TestGasPrice(t *testing.T) {
	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)
	log.Infof("Start TestGasPrice")
	gasPrice1, err := operations.GetGasPrice()
	gasPrice2 := gasPrice1
	require.NoError(t, err)
	for i := 1; i < 100; i++ {
		temp, err := operations.GetGasPrice()
		require.NoError(t, err)
		if temp > gasPrice2 {
			gasPrice2 = temp
		}
		require.NoError(t, err)

		from := common.HexToAddress(operations.DefaultL2AdminAddress)
		to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
		nonce, err := client.PendingNonceAt(ctx, from)
		require.NoError(t, err)
		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   21000,
				Value: uint256.NewInt(0),
			},
			GasPrice: uint256.NewInt(uint64(i) * 10 * encoding.Gwei),
		}
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
		require.NoError(t, err)
		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		log.Infof("Get new GP:%v, TXGP:%v", temp, tx.GetPrice())
		err = client.SendTransaction(ctx, signedTx)
		time.Sleep(500 * time.Millisecond)
		//err = WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
		//require.NoError(t, err)
		if gasPrice2 > gasPrice1 {
			log.Infof("GP compare ok: [%d,%d]", gasPrice1, gasPrice2)
			break
		}
	}
	require.NoError(t, err)
	log.Infof("gasPrice: [%d,%d]", gasPrice1, gasPrice2)
	require.Greater(t, gasPrice2, gasPrice1)
}

func TestMetrics(t *testing.T) {
	result, err := operations.GetMetricsPrometheus()
	require.NoError(t, err)
	require.Equal(t, strings.Contains(result, "sequencer_batch_execute_time"), true)
	//require.Equal(t, strings.Contains(result, "sequencer_pool_tx_count"), true)

	// TODO: enable this test after metrics are enabled
	//result, err = operations.GetMetrics()
	//require.NoError(t, err)
	//require.Equal(t, strings.Contains(result, "zkevm_getBatchWitness"), true)
	//require.Equal(t, strings.Contains(result, "eth_sendRawTransaction"), true)
	//require.Equal(t, strings.Contains(result, "eth_getTransactionCount"), true)
}

func TestMinGasPrice(t *testing.T) {
	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)
	log.Infof("Start TestMinGasPrice")
	require.NoError(t, err)
	for i := 1; i < 3; i++ {
		temp, err := operations.GetMinGasPrice()
		log.Infof("minGP: [%d]", temp)
		if temp > 1 {
			temp = temp - 1
		}
		require.NoError(t, err)

		from := common.HexToAddress(operations.DefaultL2NewAcc2Address)
		to := common.HexToAddress(operations.DefaultL1AdminAddress)
		nonce, err := client.PendingNonceAt(ctx, from)
		require.NoError(t, err)
		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   21000,
				Value: uint256.NewInt(0),
			},
			GasPrice: uint256.NewInt(temp),
		}
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2NewAcc2PrivateKey, "0x"))
		require.NoError(t, err)
		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		log.Infof("GP:%v", tx.GetPrice())
		err = client.SendTransaction(ctx, signedTx)
		require.Error(t, err)
	}
	for i := 3; i < 5; i++ {
		temp, err := operations.GetMinGasPrice()
		log.Infof("minGP: [%d]", temp)
		require.NoError(t, err)

		from := common.HexToAddress(operations.DefaultL2AdminAddress)
		to := common.HexToAddress(operations.DefaultL1AdminAddress)
		nonce, err := client.PendingNonceAt(ctx, from)
		require.NoError(t, err)
		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   21000,
				Value: uint256.NewInt(0),
			},
			GasPrice: uint256.NewInt(temp),
		}
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
		require.NoError(t, err)
		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		log.Infof("GP:%v", tx.GetPrice())
		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)
	}
	require.NoError(t, err)
}

func sendBridgeAsset(
	ctx context.Context, tokenAddr common.Address, amount *big.Int, destNetwork uint32, destAddr *common.Address,
	metadata []byte, auth *bind.TransactOpts, bridgeSCAddr common.Address, c *ethclient.Client,
) error {
	emptyAddr := common.Address{}
	if tokenAddr == emptyAddr {
		auth.Value = amount
	}
	if destAddr == nil {
		destAddr = &auth.From
	}
	if len(bridgeSCAddr) == 0 {
		return fmt.Errorf("Bridge address error")
	}

	br, err := polygonzkevmbridge.NewPolygonzkevmbridge(bridgeSCAddr, c)
	if err != nil {
		return err
	}
	tx, err := br.BridgeAsset(auth, destNetwork, *destAddr, amount, tokenAddr, true, metadata)
	if err != nil {
		return err
	}
	// wait transfer to be included in a batch
	const txTimeout = 60 * time.Second
	return operations.WaitTxToBeMined(ctx, c, tx, txTimeout)
}

type Config struct {
	HTTPMethodRateLimit string `yaml:"http.methodratelimit"`
	HTTPAPIKeys         string `yaml:"http.apikeys"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func TestSpecificProjectFreeGas(t *testing.T) {
	// transfer token to the new account
	tmpPrivateKey, err := crypto.HexToECDSA(tmpSenderPrivateKey)
	require.NoError(t, err)

	tmpPublicKey := tmpPrivateKey.Public()
	tmpPublicKeyECDSA, ok := tmpPublicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	tmpFromAddress := crypto.PubkeyToAddress(*tmpPublicKeyECDSA)
	ctx := context.Background()

	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)
	transToken(t, ctx, client,
		new(uint256.Int).Mul(uint256.NewInt(1000), uint256.NewInt(1e18)),
		tmpFromAddress.String())

	// send to specific project sender
	privateKey, err := crypto.HexToECDSA(specificProjectSenderPrivateKey)
	require.NoError(t, err)

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	transTokenWithFrom(t, ctx, client, "0x"+tmpSenderPrivateKey,
		new(uint256.Int).Mul(uint256.NewInt(100), uint256.NewInt(1e18)),
		fromAddress.String())

	erc20FreeGasAddress := common.HexToAddress(erc20FreeGasAddressStr)

	code, err := client.CodeAt(ctx, erc20FreeGasAddress, nil)
	require.NoError(t, err)
	erc20ABI, err := abi.JSON(strings.NewReader(erc20ABIJson))
	require.NoError(t, err)
	if len(code) == 0 {
		// Fetch nonce
		nonce, err := client.PendingNonceAt(ctx, fromAddress)
		require.NoError(t, err)

		log.Infof("Nonce: %d", nonce)

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

		log.Infof("Contract deployed at: %s, transaction hash: %s", erc20Address.Hex(), tx.Hash().Hex())

		// Wait for contract deployment to be mined
		bind.WaitDeployed(ctx, client, tx)
	}

	amount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // Adjust for token decimals (18 in this case)
	// Prepare transfer data
	data, err := erc20ABI.Pack("transfer", common.HexToAddress(blockAddress), amount)
	require.NoError(t, err)
	// Get the sender's nonce
	freeGasNonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	require.NoError(t, err)

	// Create the transaction with free gas
	freeGasTx := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: freeGasNonce,
			To:    &erc20FreeGasAddress,
			Gas:   60000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(0),
	}

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(freeGasTx, *signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
	err = WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
	receipt, err := client.TransactionReceipt(ctx, signedTx.Hash())
	require.NoError(t, err)
	log.Infof("receipt: %+v", receipt)

	// Send with gas price
	freeGasNonceWithGp, err := client.PendingNonceAt(context.Background(), fromAddress)
	require.NoError(t, err)
	freeGasTxWithGp := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: freeGasNonceWithGp,
			To:    &erc20FreeGasAddress,
			Gas:   60000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(100),
	}

	signerWithGp := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTxWithGp, err := types.SignTx(freeGasTxWithGp, *signerWithGp, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTxWithGp)
	require.NoError(t, err)
	err = WaitTxToBeMined(ctx, client, signedTxWithGp, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
	receiptWithGp, err := client.TransactionReceipt(ctx, signedTx.Hash())
	require.NoError(t, err)
	log.Infof("receipt: %+v", receiptWithGp)

	// send to non specific project sender
	privateKeyNon, err := crypto.HexToECDSA(nonSpecificProjectSenderPrivateKay)
	require.NoError(t, err)

	publicKeyNon := privateKeyNon.Public()
	publicKeyECDSANon, ok := publicKeyNon.(*ecdsa.PublicKey)
	require.True(t, ok)
	fromAddressNon := crypto.PubkeyToAddress(*publicKeyECDSANon)

	transTokenWithFrom(t, ctx, client, "0x"+tmpSenderPrivateKey,
		new(uint256.Int).Mul(uint256.NewInt(100), uint256.NewInt(1e18)),
		fromAddressNon.String())

	// not allowed from address
	freeGasNonceNon, err := client.PendingNonceAt(context.Background(), fromAddressNon)
	require.NoError(t, err)
	freeGasTxNon := &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: freeGasNonceNon,
			To:    &erc20FreeGasAddress,
			Gas:   60000,
			Value: uint256.NewInt(0),
			Data:  data,
		},
		GasPrice: uint256.NewInt(100),
	}

	signerNon := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTxNon, err := types.SignTx(freeGasTxNon, *signerNon, privateKeyNon)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTxNon)
	require.ErrorContains(t, err, "FEE_TOO_LOW")
}

func TestRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// latest batch seal time
	var batchNum uint64
	var batchSealTime uint64
	var err error
	for i := 0; i < 50; i++ {
		batchNum, err = operations.GetBatchNumber()
		require.NoError(t, err)
		batchSealTime, err = operations.GetBatchSealTime(new(big.Int).SetUint64(batchNum))
		require.Equal(t, batchSealTime, uint64(0))
		log.Infof("Batch number: %d, times:%v", batchNum, i)
		if batchNum > 1 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// old batch seal time
	batchNum = batchNum - 1
	batch, err := operations.GetBatchByNumber(new(big.Int).SetUint64(batchNum))
	var maxTime uint64
	for _, block := range batch.Blocks {
		blockInfo, err := operations.GetBlockByHash(common.HexToHash(block.(string)))
		require.NoError(t, err)
		log.Infof("Block Timestamp: %+v", blockInfo.Timestamp)
		blockTime := uint64(blockInfo.Timestamp)
		if blockTime > maxTime {
			maxTime = blockTime
		}
	}
	batchSealTime, err = operations.GetBatchSealTime(new(big.Int).SetUint64(batchNum))
	require.NoError(t, err)
	log.Infof("Max block time: %d, batchSealTime: %d", maxTime, batchSealTime)
	require.Equal(t, maxTime, batchSealTime)
}

func TestDebugTraceRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Wait for at least one block to be available
	var blockNumber uint64
	var err error
	for i := 0; i < 30; i++ {
		blockNumber, err = operations.GetBlockNumber()
		require.NoError(t, err)
		log.Infof("Block number: %d, attempt: %v", blockNumber, i)
		if blockNumber > 3 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.Greater(t, blockNumber, uint64(0), "Block number should be greater than 0")

	// Get a block to trace
	batchNum, err := operations.GetBatchNumber()
	require.NoError(t, err)

	batch, err := operations.GetBatchByNumber(new(big.Int).SetUint64(batchNum))
	require.NoError(t, err)
	require.NotEmpty(t, batch.Blocks, "Batch should contain at least one block")

	// Test debug_traceBlockByHash
	t.Run("DebugTraceBlockByHash", func(t *testing.T) {
		// Get the hash of the first block in the batch
		blockHash := common.HexToHash(batch.Blocks[0].(string))
		require.NotEqual(t, common.Hash{}, blockHash, "Block hash should not be empty")

		traceResult, err := operations.DebugTraceBlockByHash(blockHash)
		require.NoError(t, err)
		require.NotNil(t, traceResult, "Trace result should not be nil")

		log.Infof("DebugTraceBlockByHash result type: %T", traceResult)
	})

	// Test debug_traceBlockByNumber
	t.Run("DebugTraceBlockByNumber", func(t *testing.T) {
		traceResult, err := operations.DebugTraceBlockByNumber(1) // Trace block #1
		require.NoError(t, err)
		require.NotNil(t, traceResult, "Trace result should not be nil")

		log.Infof("DebugTraceBlockByNumber result type: %T", traceResult)
	})

	// Test debug_traceBatchByNumber
	t.Run("DebugTraceBatchByNumber", func(t *testing.T) {
		// Use batch number 1 to avoid issues with empty batches
		if batchNum > 1 {
			traceResult, err := operations.DebugTraceBatchByNumber(1)
			require.NoError(t, err)
			require.NotNil(t, traceResult, "Trace result should not be nil")

			log.Infof("DebugTraceBatchByNumber result type: %T", traceResult)
		} else {
			t.Skip("Batch number too low, skipping test")
		}
	})

	// Test debug_traceTransaction
	t.Run("DebugTraceTransaction", func(t *testing.T) {
		// Find a transaction to trace
		blockInfo, err := operations.GetBlockByHash(common.HexToHash(batch.Blocks[0].(string)))
		require.NoError(t, err)

		if len(blockInfo.Transactions) > 0 {
			// Check if we have a transaction hash directly
			if blockInfo.Transactions[0].Hash != nil {
				txHash := *blockInfo.Transactions[0].Hash
				require.NotEqual(t, common.Hash{}, txHash, "Transaction hash should not be empty")

				traceResult, err := operations.DebugTraceTransaction(txHash)
				require.NoError(t, err)
				require.NotNil(t, traceResult, "Trace result should not be nil")

				log.Infof("DebugTraceTransaction result type: %T", traceResult)
			} else {
				t.Skip("Transaction hash not available in the expected format")
			}
		} else {
			t.Skip("No transactions found in block, skipping test")
		}
	})

	// Test zkevm_getExitRootTable
	t.Run("ZKEVMGetExitRootTable", func(t *testing.T) {
		rootTable, err := operations.ZKEVMGetExitRootTable()
		require.NoError(t, err)
		require.NotNil(t, rootTable, "Exit root table should not be nil")

		log.Infof("ZKEVMGetExitRootTable result type: %T", rootTable)
	})
}

// setupTestEnvironment creates a test environment with necessary data for tests
func setupTestEnvironment(t *testing.T) (common.Hash, uint64) {
	// Wait for at least one block to be available
	var blockNumber uint64
	var err error
	for i := 0; i < 30; i++ {
		blockNumber, err = operations.GetBlockNumber()
		require.NoError(t, err)
		log.Infof("Block number: %d, attempt: %v", blockNumber, i)
		if blockNumber > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.Greater(t, blockNumber, uint64(0), "Block number should be greater than 0")

	// Get a block hash to use for tests
	batchNum, err := operations.GetBatchNumber()
	require.NoError(t, err)

	batch, err := operations.GetBatchByNumber(new(big.Int).SetUint64(batchNum))
	require.NoError(t, err)
	require.NotEmpty(t, batch.Blocks, "Batch should contain at least one block")

	blockHash := common.HexToHash(batch.Blocks[0].(string))
	require.NotEqual(t, common.Hash{}, blockHash, "Block hash should not be empty")

	return blockHash, blockNumber
}

// TestEthereumBasicRPC tests basic Ethereum RPC methods
func TestEthereumBasicRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	_, _ = setupTestEnvironment(t)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Test eth_chainId
	t.Run("EthChainID", func(t *testing.T) {
		chainID, err := operations.EthChainID()
		require.NoError(t, err)
		require.NotEqual(t, uint64(0), chainID, "Chain ID should not be zero")
		log.Infof("EthChainID result: %d", chainID)
	})

	// Test eth_syncing
	t.Run("EthSyncing", func(t *testing.T) {
		syncing, err := operations.EthSyncing()
		require.NoError(t, err)
		log.Infof("EthSyncing result: %t", syncing)
	})

	// Test eth_getBalance
	t.Run("EthGetBalance", func(t *testing.T) {
		balance, err := operations.EthGetBalance(testAddress, "latest")
		require.NoError(t, err)
		log.Infof("EthGetBalance result for test address: %s", balance.String())
	})

	// Test eth_getCode
	t.Run("EthGetCode", func(t *testing.T) {
		code, err := operations.EthGetCode(testAddress, "latest")
		require.NoError(t, err)
		log.Infof("EthGetCode result length: %d", len(code))
	})

	// Test eth_getTransactionCount
	t.Run("EthGetTransactionCount", func(t *testing.T) {
		txCount, err := operations.EthGetTransactionCount(testAddress, "latest")
		require.NoError(t, err)
		log.Infof("EthGetTransactionCount result: %d", txCount)
	})

	// Test eth_blockNumber
	t.Run("EthBlockNumber", func(t *testing.T) {
		blockNumber, err := operations.EthBlockNumber()
		require.NoError(t, err)
		require.Greater(t, blockNumber, uint64(0), "Block number should be greater than 0")
		log.Infof("EthBlockNumber result: %d", blockNumber)
	})

	// Test eth_gasPrice
	t.Run("EthGasPrice", func(t *testing.T) {
		gasPrice, err := operations.EthGasPrice()
		require.NoError(t, err)
		require.Greater(t, gasPrice.Cmp(big.NewInt(0)), 0, "Gas price should be greater than 0")
		log.Infof("EthGasPrice result: %s", gasPrice.String())
	})

	// Test eth_getStorageAt
	t.Run("EthGetStorageAt", func(t *testing.T) {
		storage, err := operations.EthGetStorageAt(testAddress, "0x0", "latest")
		require.NoError(t, err)
		log.Infof("EthGetStorageAt result: %s", storage)
	})
}

// TestEthereumBlockRPC tests Ethereum block-related RPC methods
func TestEthereumBlockRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	blockHash, blockNumber := setupTestEnvironment(t)

	// Test eth_getBlockByHash
	t.Run("EthGetBlockByHash", func(t *testing.T) {
		block, err := operations.EthGetBlockByHash(blockHash, true)
		require.NoError(t, err)
		require.NotNil(t, block, "Block should not be nil")
		log.Infof("EthGetBlockByHash result type: %T", block)
	})

	// Test eth_getBlockByNumber
	t.Run("EthGetBlockByNumber", func(t *testing.T) {
		blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
		block, err := operations.EthGetBlockByNumber(blockNumberHex, true)
		require.NoError(t, err)
		require.NotNil(t, block, "Block should not be nil")
		log.Infof("EthGetBlockByNumber result type: %T", block)
	})

	// Test eth_getBlockTransactionCountByHash
	t.Run("EthGetBlockTransactionCountByHash", func(t *testing.T) {
		txCount, err := operations.EthGetBlockTransactionCountByHash(blockHash)
		require.NoError(t, err)
		log.Infof("EthGetBlockTransactionCountByHash result: %d", txCount)
	})

	// Test eth_getBlockTransactionCountByNumber
	t.Run("EthGetBlockTransactionCountByNumber", func(t *testing.T) {
		txCount, err := operations.EthGetBlockTransactionCountByNumber("0x1") // Block #1
		require.NoError(t, err)
		log.Infof("EthGetBlockTransactionCountByNumber result: %d", txCount)
	})

	// Test eth_getTransactionByBlockHashAndIndex
	t.Run("EthGetTransactionByBlockHashAndIndex", func(t *testing.T) {
		tx, err := operations.EthGetTransactionByBlockHashAndIndex(blockHash, "0x0")
		require.NoError(t, err)
		log.Infof("EthGetTransactionByBlockHashAndIndex result type: %T", tx)
	})

	// Test eth_getTransactionByBlockNumberAndIndex
	t.Run("EthGetTransactionByBlockNumberAndIndex", func(t *testing.T) {
		tx, err := operations.EthGetTransactionByBlockNumberAndIndex("0x1", "0x0") // Block #1, first tx
		require.NoError(t, err)
		require.NotNil(t, tx, "Transaction should not be nil")
		log.Infof("EthGetTransactionByBlockNumberAndIndex result type: %T", tx)
	})

	// Test eth_getBlockInternalTransactions
	t.Run("EthGetBlockInternalTransactions", func(t *testing.T) {
		internalTxs, err := operations.EthGetBlockInternalTransactions("0x1") // Block #1
		require.NoError(t, err)
		require.NotNil(t, internalTxs, "Internal transactions should not be nil")
		log.Infof("EthGetBlockInternalTransactions result type: %T", internalTxs)
	})
}

// TestEthereumTransactionRPC tests Ethereum transaction-related RPC methods
func TestEthereumTransactionRPC(t *testing.T) {

	t.Run("EthEstimateGas", func(t *testing.T) {
		t.Skip("Skipping test due to insufficient funds")
	})

	t.Run("EthCall", func(t *testing.T) {
		t.Skip("Skipping test due to insufficient funds")
	})

	t.Run("EthGetTransactionByHash", func(t *testing.T) {
		t.Skip("Skipping test due to no available transactions")
	})

	t.Run("EthGetInternalTransactions", func(t *testing.T) {
		t.Skip("Skipping test due to no available transactions")
	})

	t.Run("EthGetTransactionReceipt", func(t *testing.T) {
		t.Skip("Skipping test due to no available transactions")
	})
}

// TestEthereumLogsRPC tests Ethereum logs-related RPC methods
func TestEthereumLogsRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Test eth_getLogs
	t.Run("EthGetLogs", func(t *testing.T) {
		fromBlock := "0x1"
		toBlock := "0x1"
		address := common.HexToAddress("0x1234567890123456789012345678901234567890")

		logs, err := operations.EthGetLogs(fromBlock, toBlock, address)
		require.NoError(t, err)
		require.NotNil(t, logs, "Logs should not be nil")
		log.Infof("EthGetLogs result type: %T", logs)
	})
}

// TestTxPoolRPC tests transaction pool related RPC methods
func TestTxPoolRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	_, _ = setupTestEnvironment(t)

	// Test txpool_content - This might return a large object, so only log type
	t.Run("TxPoolContent", func(t *testing.T) {
		content, err := operations.TxPoolContent()
		require.NoError(t, err)
		log.Infof("TxPoolContent result type: %T", content)
	})

	// Test txpool_status
	t.Run("TxPoolStatus", func(t *testing.T) {
		status, err := operations.TxPoolStatus()
		require.NoError(t, err)
		log.Infof("TxPoolStatus result type: %T", status)
	})

	// Test txpool_limbo
	t.Run("TxPoolLimbo", func(t *testing.T) {
		limbo, err := operations.TxPoolLimbo()
		require.NoError(t, err)
		require.NotNil(t, limbo, "Limbo transactions should not be nil")
		log.Infof("TxPoolLimbo result type: %T", limbo)
	})
}

// TestZKEVMRPC tests zkevm-specific RPC methods
func TestZKEVMRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	blockHash, blockNumber := setupTestEnvironment(t)

	// Test zkevm_getExitRootTable - already covered in debug tests but including here for completeness
	t.Run("ZKEVMGetExitRootTable", func(t *testing.T) {
		rootTable, err := operations.ZKEVMGetExitRootTable()
		require.NoError(t, err)
		require.NotNil(t, rootTable, "Exit root table should not be nil")
		log.Infof("ZKEVMGetExitRootTable result type: %T", rootTable)
	})

	// Test zkevm_batchNumber
	t.Run("ZKEVMBatchNumber", func(t *testing.T) {
		batchNum, err := operations.ZKEVMBatchNumber()
		require.NoError(t, err)
		require.Greater(t, batchNum, uint64(0), "Batch number should be greater than 0")
		log.Infof("ZKEVMBatchNumber result: %d", batchNum)
	})

	// Test zkevm_getLatestDataStreamBlock
	t.Run("ZKEVMGetLatestDataStreamBlock", func(t *testing.T) {
		dataStreamBlock, err := operations.ZKEVMGetLatestDataStreamBlock()
		require.NoError(t, err)
		require.NotNil(t, dataStreamBlock, "Data stream block should not be nil")
		log.Infof("ZKEVMGetLatestDataStreamBlock result type: %T", dataStreamBlock)
	})

	// Test zkevm_estimateCounters
	t.Run("ZKEVMEstimateCounters", func(t *testing.T) {
		t.Skip("Skipping test due to method handler crash")
	})

	// Test sync_getOffChainData
	t.Run("SyncGetOffChainData", func(t *testing.T) {
		t.Skip("Skipping test due to method not available")
	})

	// Test zkevm_batchNumberByBlockNumber
	t.Run("ZKEVMBatchNumberByBlockNumber", func(t *testing.T) {
		batchNum, err := operations.ZKEVMBatchNumberByBlockNumber("0x1") // Block #1
		require.NoError(t, err)
		require.Greater(t, batchNum, uint64(0), "Batch number should be greater than 0")
		log.Infof("ZKEVMBatchNumberByBlockNumber result: %d", batchNum)
	})

	// Test zkevm_getBatchByNumber
	t.Run("ZKEVMGetBatchByNumber", func(t *testing.T) {
		batch, err := operations.ZKEVMGetBatchByNumber(1) // Batch #1
		require.NoError(t, err)
		require.NotNil(t, batch, "Batch should not be nil")
		log.Infof("ZKEVMGetBatchByNumber result type: %T", batch)
	})

	// Test zkevm_getFullBlockByHash
	t.Run("ZKEVMGetFullBlockByHash", func(t *testing.T) {
		block, err := operations.ZKEVMGetFullBlockByHash(blockHash, true)
		require.NoError(t, err)
		require.NotNil(t, block, "Full block should not be nil")
		log.Infof("ZKEVMGetFullBlockByHash result type: %T", block)
	})

	// Test zkevm_getFullBlockByNumber
	t.Run("ZKEVMGetFullBlockByNumber", func(t *testing.T) {
		block, err := operations.ZKEVMGetFullBlockByNumber(blockNumber, true)
		require.NoError(t, err)
		require.NotNil(t, block, "Full block should not be nil")
		log.Infof("ZKEVMGetFullBlockByNumber result type: %T", block)
	})
}

func TestFixedNonceTooLowTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client := rtclient.NewRealtimeClient(ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)

	// Use a fixed sender address and private key
	sender := common.HexToAddress(operations.DefaultL2AdminAddress)
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)

	// Get the initial nonce
	baseNonce, err := client.PendingNonceAt(ctx, sender)
	require.NoError(t, err)
	log.Infof("Starting nonce: %d", baseNonce)

	// Fixed transaction parameters
	const (
		totalTxs       = 20 // Total number of transactions
		batchSize      = 5  // Number of transactions per batch
		nonceTooLowCnt = 5  // Fixed number of NonceTooLow transactions
	)

	// Generate transactions with a fixed pattern
	var txs []types.Transaction
	currentNonce := baseNonce
	lowNonceIndexes := map[int]bool{1: true, 4: true, 8: true, 12: true, 16: true} // Fixed low nonce transaction indices

	for i := 0; i < totalTxs; i++ {
		var nonce uint64
		if lowNonceIndexes[i] {
			// Choose a nonce that is explicitly lower than the current nonce for low nonce transactions
			nonce = baseNonce + uint64(i/4) // Ensure nonce is lower than the current valid value
		} else {
			// Normally increment nonce
			nonce = currentNonce
			currentNonce++
		}

		to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
		value := uint256.NewInt(uint64(1000 + i)) // Fixed value for easy verification
		gas := uint64(21000)
		gasPrice := uint256.NewInt(1 * encoding.Gwei) // Fixed gas price

		tx := &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   gas,
				Value: value,
				Data:  nil,
			},
			GasPrice: gasPrice,
		}

		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		txs = append(txs, signedTx)
		log.Infof("Generated tx %d: nonce=%d, value=%s", i, nonce, value.String())
	}

	// Send transactions and record results
	type txResult struct {
		tx     types.Transaction
		err    error
		sentAt time.Time
	}
	var results []txResult

	expectedNextNonce := baseNonce
	maxSuccessNonce := baseNonce - 1
	for i := 0; i < len(txs); i += batchSize {
		end := i + batchSize
		if end > len(txs) {
			end = len(txs)
		}
		batch := txs[i:end]

		// Requery nonce before sending
		currentNonce, err := client.PendingNonceAt(ctx, sender)
		if err == nil && currentNonce > expectedNextNonce {
			expectedNextNonce = currentNonce
		}
		log.Infof("Sending batch %d-%d, expected next nonce: %d", i, end-1, expectedNextNonce)

		for _, tx := range batch {
			err := client.SendTransaction(ctx, tx)
			results = append(results, txResult{
				tx:     tx,
				err:    err,
				sentAt: time.Now(),
			})
			if err != nil {
				log.Infof("Tx %s failed: %v", tx.Hash().Hex(), err)
			} else {
				log.Infof("Tx %s sent successfully", tx.Hash().Hex())
				if tx.GetNonce() > maxSuccessNonce {
					maxSuccessNonce = tx.GetNonce()
				}
				expectedNextNonce = tx.GetNonce() + 1
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Validate results
	nonceTooLowCount := 0
	successCount := 0
	for _, result := range results {
		if result.err != nil {
			errStr := result.err.Error()
			if strings.Contains(errStr, "nonce too low") || strings.Contains(errStr, "NONCE_TOO_LOW") {
				nonceTooLowCount++
				log.Infof("NonceTooLow detected: tx nonce=%d, expected next nonce=%d",
					result.tx.GetNonce(), expectedNextNonce)
				require.True(t, result.tx.GetNonce() < expectedNextNonce,
					"NonceTooLow error should occur when nonce %d < expected nonce %d",
					result.tx.GetNonce(), expectedNextNonce)
			} else if strings.Contains(errStr, "could not replace existing tx") {
				// Treat "could not replace existing tx" as a "nonce too low" scenario
				nonceTooLowCount++
				log.Infof("NonceTooLow (replacement) detected: tx nonce=%d, expected next nonce=%d",
					result.tx.GetNonce(), expectedNextNonce)
				require.True(t, result.tx.GetNonce() < expectedNextNonce,
					"NonceTooLow error should occur when nonce %d < expected nonce %d",
					result.tx.GetNonce(), expectedNextNonce)
			}
		} else {
			successCount++
			// Asynchronously verify transaction is mined
			go func(tx types.Transaction) {
				err := WaitTxToBeMined(ctx, client, tx, operations.DefaultTimeoutTxToBeMined)
				if err == nil {
					log.Debugf("Transaction mined: %s", tx.Hash().Hex())
				} else {
					log.Warnf("Transaction %s failed to be mined: %v", tx.Hash().Hex(), err)
				}
			}(result.tx)
		}
	}

	// Assert fixed results
	log.Infof("Expected NonceTooLow: %d, Actual: %d, Successful: %d",
		nonceTooLowCnt, nonceTooLowCount, successCount)

	require.Equal(t, nonceTooLowCnt, nonceTooLowCount, "NonceTooLow count does not match expectation")
	require.Equal(t, totalTxs-nonceTooLowCnt, successCount, "Number of successful transactions does not match expectation")

	// Wait for all transactions to be confirmed (optional)
	time.Sleep(5 * time.Second)

	// Validate the final nonce
	finalNonce, err := client.PendingNonceAt(ctx, sender)
	require.NoError(t, err)
	expectedFinalNonce := baseNonce + uint64(totalTxs-nonceTooLowCnt)
	require.Equal(t, expectedFinalNonce, finalNonce,
		"Final nonce is incorrect, expected: %d, actual: %d", expectedFinalNonce, finalNonce)
}

// TestVerification tests the block verification functionality
func TestVerification(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Set default verification delay batch to 2
	const defaultVerificationDelayBatch = 2

	// Helper function to get highest block number in a batch
	getHighestBlockInBatch := func(batchNumber uint64) (uint64, error) {
		batch, err := operations.GetBatchByNumber(new(big.Int).SetUint64(batchNumber))
		if err != nil {
			return 0, err
		}

		// Get blocks from batch
		blocks := batch.Blocks
		if len(blocks) == 0 {
			return 0, fmt.Errorf("no blocks found in batch %d", batchNumber)
		}

		// Find the highest block number
		var highestBlock uint64
		for _, blockHash := range blocks {
			blockHashStr, ok := blockHash.(string)
			if !ok {
				continue
			}

			block, err := operations.GetBlockByHash(common.HexToHash(blockHashStr))
			if err != nil {
				continue
			}

			blockNumber := uint64(block.Number)
			if blockNumber > highestBlock {
				highestBlock = blockNumber
			}
		}

		return highestBlock, nil
	}

	// Helper function to get finalized and safe block numbers
	getFinalizedAndSafeBlocks := func() (uint64, uint64, error) {
		// Get finalized block
		finalizedBlock, err := operations.GetBlockByNumber(big.NewInt(int64(rpc.FinalizedBlockNumber)))
		if err != nil {
			return 0, 0, err
		}
		finalizedNumber := uint64(finalizedBlock.Number)

		// Get safe block
		safeBlock, err := operations.GetBlockByNumber(big.NewInt(int64(rpc.SafeBlockNumber)))
		if err != nil {
			return 0, 0, err
		}
		safeNumber := uint64(safeBlock.Number)

		return finalizedNumber, safeNumber, nil
	}

	// Run the test 10 times
	prevBatchNumber := uint64(0)
	for i := 0; i < 10; i++ {
		log.Infof("Test iteration %d/10", i+1)

		// 1. Get current latest batch number
		latestBatchNumber, err := operations.GetBatchNumber()
		require.NoError(t, err)
		log.Infof("Latest batch number: %d", latestBatchNumber)

		// 2. Calculate target batch number (latest - default verification delay batch)
		// and also make sure that the batch number is increasing every time we check the verification
		if latestBatchNumber < defaultVerificationDelayBatch || latestBatchNumber == prevBatchNumber {
			log.Infof("Latest batch number (%d) is less than verification delay batch (%d) or equal to previous batch number (%d), skipping",
				latestBatchNumber, defaultVerificationDelayBatch, prevBatchNumber)
			time.Sleep(2 * time.Second)
			i-- // ignore this iteration
			continue
		}

		prevBatchNumber = latestBatchNumber

		targetBatchNumber := latestBatchNumber - defaultVerificationDelayBatch
		log.Infof("Target batch number: %d", targetBatchNumber)

		// 3. Get the highest block number in the target batch
		expectedVerificationBlock, err := getHighestBlockInBatch(targetBatchNumber)
		require.NoError(t, err)
		log.Infof("Expected verification block: %d", expectedVerificationBlock)

		// 4. Get finalized and safe block numbers
		finalizedBlockNumber, safeBlockNumber, err := getFinalizedAndSafeBlocks()
		require.NoError(t, err)
		log.Infof("Finalized block number: %d, Safe block number: %d", finalizedBlockNumber, safeBlockNumber)

		// 5. Wait for the finalized and safe block numbers to be >= expected verification block
		for i := 0; i < 30 && finalizedBlockNumber < expectedVerificationBlock && safeBlockNumber < expectedVerificationBlock; i++ {
			time.Sleep(1 * time.Second)
			finalizedBlockNumber, safeBlockNumber, err = getFinalizedAndSafeBlocks()
			require.NoError(t, err)
			log.Infof("Finalized block number: %d, Safe block number: %d", finalizedBlockNumber, safeBlockNumber)
		}

		// 6. Check that finalized and safe block numbers are >= highest block in target batch
		require.GreaterOrEqual(t, finalizedBlockNumber, expectedVerificationBlock,
			"Finalized block number should be >= highest block in target batch")
		require.GreaterOrEqual(t, safeBlockNumber, expectedVerificationBlock,
			"Safe block number should be >= highest block in target batch")

		log.Infof("Verification check passed for iteration %d", i+1)

		// 7. Sleep 2 seconds before next iteration
		time.Sleep(2 * time.Second)
	}

	log.Info("Verification delay batch test completed successfully")
}
