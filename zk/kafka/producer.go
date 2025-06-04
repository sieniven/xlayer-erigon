package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/kafka/types"
)

// KafkaProducer represents a Kafka producer client for sending transaction messages
type KafkaProducer struct {
	producer sarama.SyncProducer
	config   ethconfig.KafkaConfig
}

func NewKafkaProducer(config ethconfig.KafkaConfig) (*KafkaProducer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = DEFAULT_VERSION
	saramaConfig.ClientID = config.ClientID
	saramaConfig.Producer.Return.Successes = true

	// Create sync producer
	producer, err := sarama.NewSyncProducer(config.BootstrapServers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating Kafka producer: %v", err)
	}

	return &KafkaProducer{
		producer: producer,
		config:   config,
	}, nil
}

func (client *KafkaProducer) SendKafkaTransaction(ctx context.Context, blockNumber uint64, tx types.Transaction, receipt *types.Receipt) error {
	msg, err := kafkaTypes.ToKafkaTransactionMessage(tx, receipt, blockNumber)
	if err != nil {
		return fmt.Errorf("SendKafkaTransaction error: %v", err)
	}

	// Marshal message to JSON
	jsonData, err := msg.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error marshaling transaction message: %v", err)
	}

	// Create Kafka message
	kafkaMsg := &sarama.ProducerMessage{
		Topic: client.config.TxTopic,
		Value: sarama.StringEncoder(jsonData),
		Key:   sarama.StringEncoder(tx.Hash().String()),
	}

	// Send message
	_, _, err = client.producer.SendMessage(kafkaMsg)
	if err != nil {
		return fmt.Errorf("error sending message to Kafka: %v", err)
	}

	return nil
}

func (client *KafkaProducer) Close() error {
	return client.producer.Close()
}
