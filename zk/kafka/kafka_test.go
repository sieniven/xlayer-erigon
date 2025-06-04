package kafka

import (
	"context"
	"testing"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common/u256"
	types1 "github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/kafka/types"
	"github.com/ledgerwatch/log/v3"
	"gotest.tools/v3/assert"
)

var (
	testFromAddr  = libcommon.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	testToAddr    = libcommon.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	sigBytes      = "98ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4a8887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a301"
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
)

func TestKafkaConsumerGetTx(t *testing.T) {
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
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, 10)
	errorChan := make(chan error, 10)
	go consumer.ConsumeKafkaTransactions(ctx, txMsgsChan, errorChan, log.New())

	// Verify messages
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
	ctxWithCancel()
	err = consumer.Close()
	assert.NilError(t, err)
}

func TestKafkaProducerSendTx(t *testing.T) {
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
	}

	err = producer.Close()
	assert.NilError(t, err)
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
