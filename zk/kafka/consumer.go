package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
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

// ConsumeTransactions starts consuming transaction messages from the specified topic
func (client *KafkaConsumer) ConsumeTransactions(handler func(message []byte) error) error {
	// Create a partition consumer for the topic
	partitionConsumer, err := client.consumer.ConsumePartition(client.config.Topic, 0, sarama.OffsetNewest)
	if err != nil {
		return fmt.Errorf("failed to create partition consumer: %v", err)
	}
	defer partitionConsumer.Close()

	// Start consuming messages
	for {
		select {
		case msg := <-partitionConsumer.Messages():
			if err := handler(msg.Value); err != nil {
				return fmt.Errorf("error handling message: %v", err)
			}
		case err := <-partitionConsumer.Errors():
			return fmt.Errorf("error consuming message: %v", err)
		}
	}
}

func (client *KafkaConsumer) Close() error {
	return client.consumer.Close()
}
