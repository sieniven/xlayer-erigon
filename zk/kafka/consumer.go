package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/kafka/types"
)

type KafkaConsumer struct {
	consumer sarama.Consumer
	config   ethconfig.KafkaConfig
}

func NewKafkaConsumer(config ethconfig.KafkaConfig) (*KafkaConsumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = DEFAULT_VERSION
	saramaConfig.ClientID = config.ClientID

	// Create sync producer
	consumer, err := sarama.NewConsumer(config.BootstrapServers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating Kafka producer: %v", err)
	}

	return &KafkaConsumer{
		consumer: consumer,
		config:   config,
	}, nil
}

// ConsumeKafkaTransactions starts consuming transaction messages from the specified topic
func (client *KafkaConsumer) ConsumeKafkaTransactions(ctx context.Context, txMsgsChan chan kafkaTypes.TransactionMessage, errorChan chan error) {
	// Create a partition consumer for the topic
	partitionConsumer, err := client.consumer.ConsumePartition(client.config.Topic, 0, sarama.OffsetNewest)
	if err != nil {
		errorChan <- fmt.Errorf("failed to create partition consumer: %v", err)
		return
	}
	defer partitionConsumer.Close()

	// Start consuming messages
	for {
		select {
		case <-ctx.Done():
			errorChan <- fmt.Errorf("context done - stopping kafka consumer")
			return
		case msg := <-partitionConsumer.Messages():
			var txMsg kafkaTypes.TransactionMessage
			if err := json.Unmarshal(msg.Value, &txMsg); err != nil {
				errorChan <- fmt.Errorf("ConsumeKafkaTransactions error: error unmarshaling transaction message, %v", err)
				return
			}
			txMsgsChan <- txMsg
		case err := <-partitionConsumer.Errors():
			errorChan <- fmt.Errorf("ConsumeKafkaTransactions error: %v", err)
			return
		}
	}
}

func (client *KafkaConsumer) Close() error {
	return client.consumer.Close()
}
