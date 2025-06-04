package kafka

import (
	"context"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/common/u256"
	types1 "github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/kafka/types"
	"github.com/ledgerwatch/log/v3"
	"gotest.tools/v3/assert"
)

var (
	testFromAddr = libcommon.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	testToAddr   = libcommon.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	sigBytes     = "98ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4a8887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a301"

	rightvrsTx, _ = types1.NewTransaction(
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

	rightvrsTxReceipt = &types1.Receipt{
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

	difficulty, _ = new(big.Int).SetString("8398142613866510000000000000000000000000000000", 10)
	blockHeader   = &types1.Header{
		ParentHash:  libcommon.HexToHash("0x8b00fcf1e541d371a3a1b79cc999a85cc3db5ee5637b5159646e1acd3613fd15"),
		UncleHash:   libcommon.HexToHash("1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"),
		Coinbase:    libcommon.HexToAddress("0x571846e42308df2dad8ed792f44a8bfddf0acb4d"),
		Root:        libcommon.HexToHash("0x351780124dae86b84998c6d4fe9a88acfb41b4856b4f2c56767b51a4e2f94dd4"),
		TxHash:      libcommon.HexToHash("0x6a35133fbff7ea2cb5ee7635c9fb623f96d31d689d806a2bfe40a2b1d90ee99c"),
		ReceiptHash: libcommon.HexToHash("0x324f54860e214ea896ea7a05bda30f85541be3157de77a9059a04fdb1e86badd"),
		Difficulty:  difficulty,
		Number:      big.NewInt(24679923),
		GasLimit:    30_000_000,
		GasUsed:     3_074_345,
		Time:        1666343339,
		Extra:       common.FromHex("0x1234"),
		BaseFee:     big.NewInt(7_000_000_000),
		AuRaStep:    13078,
		AuRaSeal:    common.FromHex("0x75bda30f85541be059646e1acd3613fd100846e42308df2dad8ed79b9a9e91c9db994386599a683820a1394684d41fc139c4805684142e6b15a722a2e9cc51f7ee"),
	}
)

func TestKafkaConsumer(t *testing.T) {
	rightvrsTx.SetSender(testFromAddr)
	cfg := ethconfig.KafkaConfig{
		Enable:           true,
		BootstrapServers: []string{"0.0.0.0:9094"},
		BlockTopic:       "xlayer-test-block",
		TxTopic:          "xlayer-test-tx",
		ClientID:         "xlayer-test-consumer",
	}
	consumer, err := NewKafkaConsumer(cfg)
	assert.NilError(t, err)
	ctx, ctxWithCancel := context.WithCancel(context.Background())
	headersChan := make(chan types1.Header, 10)
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, 10)
	errorChan := make(chan error, 10)
	go consumer.ConsumeKafka(ctx, headersChan, txMsgsChan, errorChan, log.New())

	// Verify tx messages
	for i := 0; i < 10; i++ {
		select {
		case err := <-errorChan:
			t.Fatalf("Received error from consumer: %v", err)
		case txMsg := <-txMsgsChan:
			assert.Equal(t, txMsg.BlockNumber, uint64(i))
			assert.Equal(t, int(txMsg.Type), types1.LegacyTxType)
			assert.Equal(t, txMsg.Hash, rightvrsTx.Hash().String())
			assert.Equal(t, txMsg.From, testFromAddr.String())
			assert.Equal(t, txMsg.ChainID, rightvrsTx.GetChainID().Uint64())
			assert.Equal(t, txMsg.Nonce, rightvrsTx.GetNonce())
			assert.Equal(t, txMsg.Gas, uint64(2000))
			assert.Equal(t, txMsg.To, testToAddr.String())
			assert.Equal(t, txMsg.Value, "10")
			assert.Equal(t, txMsg.Data, "5544")
			v, r, s := rightvrsTx.RawSignatureValues()
			assert.Equal(t, txMsg.R, r.Hex())
			assert.Equal(t, txMsg.S, s.Hex())
			assert.Equal(t, txMsg.V, v.Hex())
			assert.Equal(t, txMsg.GasPrice, "1")
			assertReceipt(t, txMsg, rightvrsTxReceipt)
		}
	}

	// Verify header messages
	for i := 0; i < 10; i++ {
		select {
		case err := <-errorChan:
			t.Fatalf("Received error from consumer: %v", err)
		case rcvHeader := <-headersChan:
			assertHeader(t, blockHeader, &rcvHeader)
		}
	}

	ctxWithCancel()
	err = consumer.Close()
	assert.NilError(t, err)
}

func TestKafkaProducer(t *testing.T) {
	rightvrsTx.SetSender(testFromAddr)
	cfg := ethconfig.KafkaConfig{
		Enable:           true,
		BootstrapServers: []string{"0.0.0.0:9094"},
		BlockTopic:       "xlayer-test-block",
		TxTopic:          "xlayer-test-tx",
		ClientID:         "xlayer-test-consumer",
	}
	producer, err := NewKafkaProducer(cfg)
	assert.NilError(t, err)

	for i := 0; i < 10; i++ {
		err = producer.SendKafkaTransaction(context.Background(), uint64(i), rightvrsTx, rightvrsTxReceipt)
		assert.NilError(t, err)

		err = producer.SendKafkaBlockHeader(context.Background(), blockHeader)
		assert.NilError(t, err)
	}

	err = producer.Close()
	assert.NilError(t, err)
}

func assertHeader(t *testing.T, header *types1.Header, rcvHeader *types1.Header) {
	assert.Equal(t, header.ParentHash, rcvHeader.ParentHash)
	assert.Equal(t, header.UncleHash, rcvHeader.UncleHash)
	assert.Equal(t, header.Coinbase, rcvHeader.Coinbase)
	assert.Equal(t, header.Root, rcvHeader.Root)
	assert.Equal(t, header.TxHash, rcvHeader.TxHash)
	assert.Equal(t, header.ReceiptHash, rcvHeader.ReceiptHash)
	assert.Equal(t, header.Bloom, rcvHeader.Bloom)
	assert.Equal(t, header.Number.String(), rcvHeader.Number.String())
	assert.Equal(t, header.Difficulty.String(), rcvHeader.Difficulty.String())
	assert.Equal(t, header.GasLimit, rcvHeader.GasLimit)
	assert.Equal(t, header.GasUsed, rcvHeader.GasUsed)
	assert.Equal(t, header.Time, rcvHeader.Time)
	assert.Equal(t, string(header.Extra), string(rcvHeader.Extra))
	assert.Equal(t, header.BaseFee.String(), rcvHeader.BaseFee.String())
	assert.Equal(t, header.AuRaStep, rcvHeader.AuRaStep)
	assert.Equal(t, string(header.AuRaSeal), string(rcvHeader.AuRaSeal))
	assert.Equal(t, header.BlobGasUsed, rcvHeader.BlobGasUsed)
	assert.Equal(t, header.ExcessBlobGas, rcvHeader.ExcessBlobGas)
}

func assertReceipt(t *testing.T, msg kafkaTypes.TransactionMessage, receipt *types1.Receipt) {
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
