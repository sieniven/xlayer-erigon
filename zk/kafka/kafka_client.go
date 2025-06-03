package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"

	"github.com/IBM/sarama"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/log/v3"
)

// KafkaConfig holds the configuration for the Kafka client
type KafkaConfig struct {
	Brokers    []string
	Topic      string
	ClientID   string
	Version    string
	Username   string
	Password   string
	RootCAPath string
}

// KafkaClient represents a Kafka client for sending transaction messages
type KafkaClient struct {
	producer sarama.SyncProducer
	config   KafkaConfig
	logger   log.Logger
}

// newTLSConfig creates a new TLS configuration for Kafka
func newTLSConfig(rootCAPath string) (*tls.Config, error) {
	rootCA, err := os.ReadFile(rootCAPath)
	if err != nil {
		return nil, fmt.Errorf("error reading root CA file: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(rootCA); !ok {
		return nil, fmt.Errorf("failed to append root CA to cert pool")
	}

	return &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}, nil
}

// NewKafkaClient creates a new Kafka client instance
func NewKafkaClient(config KafkaConfig, logger log.Logger) (*KafkaClient, error) {
	saramaConfig := sarama.NewConfig()

	// Set Kafka version
	version, err := sarama.ParseKafkaVersion(config.Version)
	if err != nil {
		return nil, fmt.Errorf("error parsing Kafka version: %v", err)
	}
	saramaConfig.Version = version

	// Set client ID
	saramaConfig.ClientID = config.ClientID

	// Configure SASL if credentials are provided
	if config.Username != "" && config.Password != "" {
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		saramaConfig.Net.SASL.User = config.Username
		saramaConfig.Net.SASL.Password = config.Password
	}

	// Configure TLS if RootCA is provided
	if config.RootCAPath != "" {
		tlsConfig, err := newTLSConfig(config.RootCAPath)
		if err != nil {
			return nil, fmt.Errorf("error creating TLS config: %v", err)
		}
		saramaConfig.Net.TLS.Enable = true
		saramaConfig.Net.TLS.Config = tlsConfig
	}

	// Create sync producer
	producer, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating Kafka producer: %v", err)
	}

	return &KafkaClient{
		producer: producer,
		config:   config,
		logger:   logger,
	}, nil
}

// SendTransaction sends a transaction message to Kafka
func (k *KafkaClient) SendTransaction(ctx context.Context, tx types.Transaction, blockNumber uint64, blockHash string, blockTime uint64) error {
	msg := TransactionMessage{
		Type:        tx.Type(),
		ChainID:     tx.GetChainID().Uint64(),
		Nonce:       tx.GetNonce(),
		GasPrice:    tx.GetPrice().String(),
		Gas:         tx.GetGas(),
		To:          tx.GetTo().String(),
		Value:       tx.GetValue().String(),
		Data:        fmt.Sprintf("0x%x", tx.GetData()),
		V:           tx.RawSignatureValues().V.String(),
		R:           tx.RawSignatureValues().R.String(),
		S:           tx.RawSignatureValues().S.String(),
		Hash:        tx.Hash().String(),
		From:        tx.GetSender().String(),
		BlockNumber: blockNumber,
	}

	// Add type-specific fields
	switch tx.Type() {
	case types.AccessListTxType:
		msg.AccessList = tx.GetAccessList()
	case types.DynamicFeeTxType:
		msg.MaxFeePerGas = tx.GetFeeCap().String()
		msg.MaxPriorityFeePerGas = tx.GetTip().String()
	case types.BlobTxType:
		msg.MaxFeePerGas = tx.GetFeeCap().String()
		msg.MaxPriorityFeePerGas = tx.GetTip().String()
		msg.BlobGasFeeCap = tx.GetBlobGasFeeCap().String()
		msg.BlobHashes = make([]string, len(tx.GetBlobHashes()))
		for i, hash := range tx.GetBlobHashes() {
			msg.BlobHashes[i] = hash.String()
		}
	}

	// Marshal message to JSON
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error marshaling transaction message: %v", err)
	}

	// Create Kafka message
	kafkaMsg := &sarama.ProducerMessage{
		Topic: k.config.Topic,
		Value: sarama.StringEncoder(jsonData),
		Key:   sarama.StringEncoder(tx.Hash().String()),
	}

	// Send message
	_, _, err = k.producer.SendMessage(kafkaMsg)
	if err != nil {
		return fmt.Errorf("error sending message to Kafka: %v", err)
	}

	return nil
}

// Close closes the Kafka producer
func (k *KafkaClient) Close() error {
	return k.producer.Close()
}
