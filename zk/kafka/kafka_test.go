package kafka

import (
	"testing"

	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"gotest.tools/v3/assert"
)

func TestKafkaConsumer(t *testing.T) {
	cfg := ethconfig.KafkaConfig{
		Enable:           true,
		BootstrapServers: []string{"0.0.0.0:9094"},
		Topic:            "test",
		ClientID:         "test",
	}
	_, err := NewKafkaConsumer(cfg)
	assert.NilError(t, err)
}

func TestKafkaProducer(t *testing.T) {
	cfg := ethconfig.KafkaConfig{
		Enable:           true,
		BootstrapServers: []string{"0.0.0.0:9094"},
		Topic:            "test",
		ClientID:         "test",
	}
	_, err := NewKafkaProducer(cfg)
	assert.NilError(t, err)
}
