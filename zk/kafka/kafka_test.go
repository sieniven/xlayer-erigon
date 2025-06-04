package kafka

import (
	"context"
	"testing"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common/u256"
	types1 "github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"gotest.tools/v3/assert"
)

var (
	testFromAddr = libcommon.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
	testToAddr   = libcommon.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	addr         = libcommon.HexToAddress("0x0000000000000000000000000000000000000001")
)

func TestKafkaConsumer(t *testing.T) {
	cfg := ethconfig.KafkaConfig{
		Enable:           true,
		BootstrapServers: []string{"0.0.0.0:9094"},
		Topic:            "xlayer-test",
		ClientID:         "xlayer-test-consumer",
	}
	_, err := NewKafkaConsumer(cfg)
	assert.NilError(t, err)
}

func TestKafkaProducer(t *testing.T) {
	cfg := ethconfig.KafkaConfig{
		Enable:           true,
		BootstrapServers: []string{"0.0.0.0:9094"},
		Topic:            "xlayer-test",
		ClientID:         "xlayer-test-producer",
	}
	producer, err := NewKafkaProducer(cfg)
	assert.NilError(t, err)

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

	for i := 0; i < 10; i++ {
		err = producer.SendKafkaTransaction(context.Background(), uint64(i), rightvrsTx, rightvrsTxReceipt)
		assert.NilError(t, err)
	}

	err = producer.Close()
	assert.NilError(t, err)
}
