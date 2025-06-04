package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/kafka/types"
	"github.com/ledgerwatch/log/v3"
)

type KafkaConsumer struct {
	consumer sarama.ConsumerGroup
	config   ethconfig.KafkaConfig
}

func NewKafkaConsumer(config ethconfig.KafkaConfig) (*KafkaConsumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = DEFAULT_VERSION
	saramaConfig.ClientID = config.ClientID
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	saramaConfig.Consumer.Offsets.AutoCommit.Enable = true

	// Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup(config.BootstrapServers, config.ClientID, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating Kafka consumer: %v", err)
	}

	return &KafkaConsumer{
		consumer: consumerGroup,
		config:   config,
	}, nil
}

type consumerGroupHandler struct {
	ctx        context.Context
	txMsgsChan chan kafkaTypes.TransactionMessage
	errorChan  chan error
	logger     log.Logger
	txTopic    string
	blockTopic string
}

func (h *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	h.logger.Info("Starting kafka consumption for topic %s partition %d offset %d", claim.Topic(), claim.Partition(), claim.InitialOffset())
	for {
		select {
		case <-h.ctx.Done():
			err := fmt.Errorf("context cancelled - stopping consume claim")
			h.errorChan <- err
			return err
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			switch msg.Topic {
			case h.blockTopic:
			case h.txTopic:
				var txMsg kafkaTypes.TransactionMessage
				if err := json.Unmarshal(msg.Value, &txMsg); err != nil {
					h.errorChan <- fmt.Errorf("consume claim error, unmarshaling transaction message: %v", err)
					return err
				}

				// Send message to channel
				select {
				case h.txMsgsChan <- txMsg:
					session.MarkMessage(msg, "")
				case <-h.ctx.Done():
					err := fmt.Errorf("context cancelled - stopping consume claim")
					h.errorChan <- err
					return err
				}
			default:
				err := fmt.Errorf("unknown topic: %s", msg.Topic)
				h.errorChan <- err
				return err
			}
		}
	}
}

// ConsumeKafkaTransactions starts consuming transaction messages from the specified topic
func (client *KafkaConsumer) ConsumeKafkaTransactions(ctx context.Context, txMsgsChan chan kafkaTypes.TransactionMessage, errorChan chan error, logger log.Logger) {
	handler := &consumerGroupHandler{
		ctx:        ctx,
		txMsgsChan: txMsgsChan,
		errorChan:  errorChan,
		logger:     logger,
		txTopic:    client.config.TxTopic,
		blockTopic: client.config.BlockTopic,
	}

	topics := []string{client.config.TxTopic, client.config.BlockTopic}
	err := client.consumer.Consume(ctx, topics, handler)
	if err != nil {
		errorChan <- fmt.Errorf("ConsumeKafkaTransactions error: %v", err)
		return
	}
}

func (client *KafkaConsumer) Close() error {
	return client.consumer.Close()
}
