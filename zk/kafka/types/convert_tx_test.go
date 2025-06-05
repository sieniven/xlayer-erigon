package types

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	types2 "github.com/ledgerwatch/erigon-lib/types"
	"github.com/ledgerwatch/erigon/common/u256"
	types1 "github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/core/vm"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
	"gotest.tools/v3/assert"
)

var (
	testFromAddr = libcommon.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	testToAddr   = libcommon.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	addr         = libcommon.HexToAddress("0x0000000000000000000000000000000000000001")
	accesses     = types2.AccessList{{Address: addr, StorageKeys: []libcommon.Hash{{0}}}}

	dynFeeTx = &types1.DynamicFeeTransaction{
		CommonTx: types1.CommonTx{
			Nonce: 3,
			To:    &testToAddr,
			Value: uint256.NewInt(10),
			Gas:   25000,
			Data:  libcommon.FromHex("5544"),
		},
		ChainID:    u256.Num1,
		Tip:        uint256.NewInt(1),
		FeeCap:     uint256.NewInt(1),
		AccessList: accesses,
	}

	blobTx = &types1.BlobTx{
		DynamicFeeTransaction: *dynFeeTx,
		MaxFeePerBlobGas:      uint256.NewInt(10),
		BlobVersionedHashes:   []libcommon.Hash{{0}},
	}
)

func TestLegacyTx(t *testing.T) {
	// Test from
	emptyTx := types1.NewTransaction(
		0,
		libcommon.HexToAddress(testToAddr.String()),
		uint256.NewInt(0), 0, uint256.NewInt(10),
		nil,
	)
	emptyTx.SetSender(testFromAddr)

	emptyTxReceipt := types1.NewReceipt(false, 1000)

	blockNumber := uint64(100)
	emptyMsg, err := ToKafkaTransactionMessage(emptyTx, emptyTxReceipt, nil, blockNumber)
	assert.NilError(t, err)
	assertCommonTx(t, emptyMsg, emptyTx, blockNumber, types1.LegacyTxType)
	assert.Equal(t, emptyMsg.GasPrice, emptyTx.GetPrice().String())
	assertReceipt(t, emptyMsg, emptyTxReceipt)
	assertInnerTxs(t, emptyMsg, nil)

	sigBytes := "98ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4a8887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a301"
	rightvrsTx, _ := types1.NewTransaction(
		3,
		testToAddr,
		uint256.NewInt(10),
		2000,
		u256.Num1,
		libcommon.FromHex("5544"),
	).WithSignature(
		*types1.LatestSignerForChainID(nil),
		libcommon.Hex2Bytes(sigBytes),
	)
	rightvrsTx.SetSender(testFromAddr)

	rightvrsTxReceipt := &types1.Receipt{
		PostState:         libcommon.Hash{2}.Bytes(),
		CumulativeGasUsed: 3,
		Logs: []*types1.Log{
			{Address: libcommon.BytesToAddress([]byte{0x22})},
			{Address: libcommon.BytesToAddress([]byte{0x02, 0x22})},
		},
		TxHash:          rightvrsTx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x02, 0x22, 0x22}),
		GasUsed:         2,
	}

	rightvrsTxInnerTxs := []*zktypes.InnerTx{
		{
			Name:     "innerTx1",
			CallType: vm.CALL_TYP,
		},
	}

	msg, err := ToKafkaTransactionMessage(rightvrsTx, rightvrsTxReceipt, rightvrsTxInnerTxs, blockNumber)
	assert.NilError(t, err)
	assertCommonTx(t, msg, rightvrsTx, blockNumber, types1.LegacyTxType)
	assert.Equal(t, msg.GasPrice, rightvrsTx.GetPrice().String())
	assertReceipt(t, msg, rightvrsTxReceipt)
	assertInnerTxs(t, msg, rightvrsTxInnerTxs)

	// Test to
	convertEmptyTx, convertBlockNumber, err := emptyMsg.GetTransaction()
	assert.NilError(t, err)
	assert.Equal(t, convertBlockNumber, blockNumber)
	assertCommonTx(t, emptyMsg, convertEmptyTx, convertBlockNumber, types1.LegacyTxType)
	assert.Equal(t, emptyMsg.GasPrice, convertEmptyTx.GetPrice().String())

	convertRightvsTx, convertBlockNumber, err := msg.GetTransaction()
	assert.NilError(t, err)
	assert.Equal(t, convertBlockNumber, blockNumber)
	assertCommonTx(t, msg, convertRightvsTx, convertBlockNumber, types1.LegacyTxType)
	assert.Equal(t, msg.GasPrice, convertRightvsTx.GetPrice().String())
}

func TestAccessListTx(t *testing.T) {
	// Test from
	sigBytes := "c9519f4f2b30335884581971573fadf60c6204f59a911df35ee8a540456b266032f1e8e2c5dd761f9e4f88f41c8310aeaba26a8bfcdacfedfa12ec3862d3752101"
	accessListTx := &types1.AccessListTx{
		ChainID: u256.Num1,
		LegacyTx: types1.LegacyTx{
			CommonTx: types1.CommonTx{
				Nonce: 3,
				To:    &testToAddr,
				Value: uint256.NewInt(10),
				Gas:   25000,
				Data:  libcommon.FromHex("5544"),
			},
			GasPrice: uint256.NewInt(1),
		},
		AccessList: accesses,
	}

	signedAccessListTx, _ := accessListTx.WithSignature(
		*types1.LatestSignerForChainID(big.NewInt(1)),
		libcommon.Hex2Bytes(sigBytes),
	)
	signedAccessListTx.SetSender(testFromAddr)

	signedAccessListTxReceipt := &types1.Receipt{
		Type:              types1.AccessListTxType,
		PostState:         libcommon.Hash{3}.Bytes(),
		CumulativeGasUsed: 6,
		Logs: []*types1.Log{
			{Address: libcommon.BytesToAddress([]byte{0x33})},
			{Address: libcommon.BytesToAddress([]byte{0x03, 0x33})},
		},
		TxHash:          signedAccessListTx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x03, 0x33, 0x33}),
		GasUsed:         3,
	}
	signedAccessListTxInnerTxs := []*zktypes.InnerTx{
		{
			Name:     "innerTx1",
			CallType: vm.CALL_TYP,
		},
	}

	blockNumber := uint64(100)
	msg, err := ToKafkaTransactionMessage(signedAccessListTx, signedAccessListTxReceipt, signedAccessListTxInnerTxs, blockNumber)
	assert.NilError(t, err)
	assertCommonTx(t, msg, signedAccessListTx, blockNumber, types1.AccessListTxType)
	assert.Equal(t, msg.GasPrice, signedAccessListTx.GetPrice().String())
	assertReceipt(t, msg, signedAccessListTxReceipt)
	assertInnerTxs(t, msg, signedAccessListTxInnerTxs)
	assertAccessList(t, msg.AccessList)

	// Test to
	convertAccessListTx, convertBlockNumber, err := msg.GetTransaction()
	assert.NilError(t, err)
	assert.Equal(t, convertBlockNumber, blockNumber)
	assertCommonTx(t, msg, convertAccessListTx, convertBlockNumber, types1.AccessListTxType)
	assertTxAccessList(t, convertAccessListTx.GetAccessList())
	assert.Equal(t, msg.GasPrice, convertAccessListTx.GetPrice().String())
}

func TestDynamicFeeTx(t *testing.T) {
	// Test from
	sigBytes := "c9519f4f2b30335884581971573fadf60c6204f59a911df35ee8a540456b266032f1e8e2c5dd761f9e4f88f41c8310aeaba26a8bfcdacfedfa12ec3862d3752101"
	signedDynFeeTx, _ := dynFeeTx.WithSignature(
		*types1.LatestSignerForChainID(big.NewInt(1)),
		libcommon.Hex2Bytes(sigBytes),
	)
	signedDynFeeTx.SetSender(testFromAddr)

	signedDynFeeTxReceipt := &types1.Receipt{
		Type:              types1.DynamicFeeTxType,
		PostState:         libcommon.Hash{4}.Bytes(),
		CumulativeGasUsed: 10,
		Logs: []*types1.Log{
			{Address: libcommon.BytesToAddress([]byte{0x33})},
			{Address: libcommon.BytesToAddress([]byte{0x03, 0x33})},
		},
		TxHash:          signedDynFeeTx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x03, 0x33, 0x33}),
		GasUsed:         3,
	}
	signedDynFeeTxInnerTxs := []*zktypes.InnerTx{
		{
			Name:     "innerTx1",
			CallType: vm.CALL_TYP,
		},
	}

	blockNumber := uint64(100)
	msg, err := ToKafkaTransactionMessage(signedDynFeeTx, signedDynFeeTxReceipt, signedDynFeeTxInnerTxs, blockNumber)
	assert.NilError(t, err)
	assertCommonTx(t, msg, signedDynFeeTx, blockNumber, types1.DynamicFeeTxType)
	assert.Equal(t, msg.Tip, signedDynFeeTx.GetTip().String())
	assert.Equal(t, msg.FeeCap, signedDynFeeTx.GetFeeCap().String())
	assertReceipt(t, msg, signedDynFeeTxReceipt)
	assertInnerTxs(t, msg, signedDynFeeTxInnerTxs)
	assertAccessList(t, msg.AccessList)

	// Test to
	convertDynFeeTx, convertBlockNumber, err := msg.GetTransaction()
	assert.NilError(t, err)
	assert.Equal(t, convertBlockNumber, blockNumber)
	assertCommonTx(t, msg, convertDynFeeTx, convertBlockNumber, types1.DynamicFeeTxType)
	assertTxAccessList(t, convertDynFeeTx.GetAccessList())
	assert.Equal(t, msg.Tip, convertDynFeeTx.GetTip().String())
	assert.Equal(t, msg.FeeCap, convertDynFeeTx.GetFeeCap().String())
}

func TestFromBlobTx(t *testing.T) {
	// Test from
	blobTx.SetSender(testFromAddr)
	blobTxReceipt := &types1.Receipt{
		PostState:         libcommon.Hash{2}.Bytes(),
		CumulativeGasUsed: 15,
		Logs: []*types1.Log{
			{Address: libcommon.BytesToAddress([]byte{0x22})},
			{Address: libcommon.BytesToAddress([]byte{0x02, 0x22})},
		},
		TxHash:          blobTx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x02, 0x22, 0x22}),
		GasUsed:         5,
	}
	blobTxInnerTxs := []*zktypes.InnerTx{
		{
			Name:     "innerTx1",
			CallType: vm.CALL_TYP,
		},
	}

	blockNumber := uint64(100)
	msg, err := ToKafkaTransactionMessage(blobTx, blobTxReceipt, blobTxInnerTxs, blockNumber)
	assert.NilError(t, err)
	assertCommonTx(t, msg, blobTx, blockNumber, types1.BlobTxType)
	assert.Equal(t, msg.Tip, blobTx.GetTip().String())
	assert.Equal(t, msg.FeeCap, blobTx.GetFeeCap().String())
	assertReceipt(t, msg, blobTxReceipt)
	assertInnerTxs(t, msg, blobTxInnerTxs)
	assertAccessList(t, msg.AccessList)

	assert.Equal(t, msg.MaxFeePerBlobGas, "10")
	assert.Equal(t, len(msg.BlobVersionedHashes), 1)
	for _, hash := range msg.BlobVersionedHashes {
		assert.Equal(t, hash, "0x0000000000000000000000000000000000000000000000000000000000000000")
	}

	// Test to
	convertBlobTx, convertBlockNumber, err := msg.GetTransaction()
	assert.NilError(t, err)
	assertCommonTx(t, msg, convertBlobTx, convertBlockNumber, types1.BlobTxType)
	assert.Equal(t, msg.Tip, convertBlobTx.GetTip().String())
	assert.Equal(t, msg.FeeCap, convertBlobTx.GetFeeCap().String())
	assertTxAccessList(t, convertBlobTx.GetAccessList())
}

func assertCommonTx(t *testing.T, msg TransactionMessage, tx types1.Transaction, blockNumber uint64, txType int) {
	assert.Equal(t, msg.BlockNumber, blockNumber)
	assert.Equal(t, int(msg.Type), txType)
	assert.Equal(t, msg.Hash, tx.Hash())
	assert.Equal(t, msg.From, testFromAddr)
	assert.Equal(t, msg.ChainID.Uint64(), tx.GetChainID().Uint64())
	assert.Equal(t, msg.Nonce, tx.GetNonce())
	assert.Equal(t, msg.Gas, tx.GetGas())
	assert.Equal(t, msg.To.String(), testToAddr.String())
	assert.Equal(t, msg.Value.String(), tx.GetValue().String())
	assert.Equal(t, string(msg.Data), string(tx.GetData()))
	v, r, s := tx.RawSignatureValues()
	assert.Equal(t, msg.R, *r)
	assert.Equal(t, msg.S, *s)
	assert.Equal(t, msg.V, *v)
}

func assertAccessList(t *testing.T, msgAccessList []AccessTupleMessage) {
	assert.Equal(t, len(msgAccessList), len(accesses))
	for idx, access := range msgAccessList {
		assert.Equal(t, access.Address, accesses[idx].Address.String())
		assert.Equal(t, len(access.StorageKeys), len(accesses[idx].StorageKeys))
		for i, storageKey := range access.StorageKeys {
			assert.Equal(t, storageKey, accesses[idx].StorageKeys[i].Hex())
		}
	}
}

func assertTxAccessList(t *testing.T, accessList types2.AccessList) {
	assert.Equal(t, len(accessList), len(accesses))
	for idx, access := range accessList {
		assert.Equal(t, access.Address, accesses[idx].Address)
		assert.Equal(t, len(access.StorageKeys), len(accesses[idx].StorageKeys))
		for i, storageKey := range access.StorageKeys {
			assert.Equal(t, storageKey, accesses[idx].StorageKeys[i])
		}
	}
}

func assertReceipt(t *testing.T, msg TransactionMessage, receipt *types1.Receipt) {
	assert.Equal(t, msg.Receipt.Type, receipt.Type)
	assert.Equal(t, string(msg.Receipt.PostState), string(receipt.PostState))
	assert.Equal(t, msg.Receipt.Status, receipt.Status)
	assert.Equal(t, msg.Receipt.CumulativeGasUsed, receipt.CumulativeGasUsed)
	assert.Equal(t, msg.Receipt.Bloom, receipt.Bloom)
	assert.Equal(t, len(msg.Receipt.Logs), len(receipt.Logs))
	for i := range msg.Receipt.Logs {
		assert.Equal(t, msg.Receipt.Logs[i].Address.String(), receipt.Logs[i].Address.String())
		assert.Equal(t, len(msg.Receipt.Logs[i].Topics), len(receipt.Logs[i].Topics))
		for j := range msg.Receipt.Logs[i].Topics {
			assert.Equal(t, msg.Receipt.Logs[i].Topics[j].String(), receipt.Logs[i].Topics[j].String())
		}
		assert.Equal(t, string(msg.Receipt.Logs[i].Data), string(receipt.Logs[i].Data))
	}

	assert.Equal(t, msg.Receipt.TxHash, receipt.TxHash)
	assert.Equal(t, msg.Receipt.ContractAddress.String(), receipt.ContractAddress.String())
	assert.Equal(t, msg.Receipt.GasUsed, receipt.GasUsed)
	assert.Equal(t, msg.Receipt.BlockHash, receipt.BlockHash)
	assert.Equal(t, msg.Receipt.BlockNumber, receipt.BlockNumber)
	assert.Equal(t, msg.Receipt.TransactionIndex, receipt.TransactionIndex)
}

//	type InnerTx struct {
//		Dept          big.Int `json:"dept"`
//		InternalIndex big.Int `json:"internal_index"`
//		CallType      string  `json:"call_type"`
//		Name          string  `json:"name"`
//		TraceAddress  string  `json:"trace_address"`
//		CodeAddress   string  `json:"code_address"`
//		From          string  `json:"from"`
//		To            string  `json:"to"`
//		Input         string  `json:"input"`
//		Output        string  `json:"output"`
//		IsError       bool    `json:"is_error"`
//		Gas           uint64  `json:"gas"`
//		GasUsed       uint64  `json:"gas_used"`
//		Value         string  `json:"value"`
//		ValueWei      string  `json:"value_wei"`
//		CallValueWei  string  `json:"call_value_wei"`
//		Error         string  `json:"error"`
//	}
func assertInnerTxs(t *testing.T, msg TransactionMessage, innerTxs []*zktypes.InnerTx) {
	assert.Equal(t, len(msg.InnerTxs), len(innerTxs))
	for i := range msg.InnerTxs {
		assert.Equal(t, msg.InnerTxs[i].Dept.String(), innerTxs[i].Dept.String())
		assert.Equal(t, msg.InnerTxs[i].InternalIndex.String(), innerTxs[i].InternalIndex.String())
		assert.Equal(t, msg.InnerTxs[i].CallType, innerTxs[i].CallType)
		assert.Equal(t, msg.InnerTxs[i].Name, innerTxs[i].Name)
		assert.Equal(t, msg.InnerTxs[i].TraceAddress, innerTxs[i].TraceAddress)
		assert.Equal(t, msg.InnerTxs[i].CodeAddress, innerTxs[i].CodeAddress)
		assert.Equal(t, msg.InnerTxs[i].From, innerTxs[i].From)
		assert.Equal(t, msg.InnerTxs[i].To, innerTxs[i].To)
		assert.Equal(t, msg.InnerTxs[i].Input, innerTxs[i].Input)
		assert.Equal(t, msg.InnerTxs[i].Output, innerTxs[i].Output)
		assert.Equal(t, msg.InnerTxs[i].IsError, innerTxs[i].IsError)
		assert.Equal(t, msg.InnerTxs[i].Gas, innerTxs[i].Gas)
		assert.Equal(t, msg.InnerTxs[i].GasUsed, innerTxs[i].GasUsed)
		assert.Equal(t, msg.InnerTxs[i].Value, innerTxs[i].Value)
		assert.Equal(t, msg.InnerTxs[i].ValueWei, innerTxs[i].ValueWei)
		assert.Equal(t, msg.InnerTxs[i].CallValueWei, innerTxs[i].CallValueWei)
		assert.Equal(t, msg.InnerTxs[i].Error, innerTxs[i].Error)
	}
}
