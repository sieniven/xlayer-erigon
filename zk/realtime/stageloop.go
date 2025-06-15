package realtime

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/zk/realtime/kafka"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/realtime/kafka/types"
	realtimeTypes "github.com/ledgerwatch/erigon/zk/realtime/types"
	"github.com/ledgerwatch/erigon/zk/sequencer"
	"github.com/ledgerwatch/log/v3"
)

var (
	readyFlag         = atomic.Bool{}
	errorFlag         = atomic.Bool{}
	resetFlag         = atomic.Bool{}
	kafkaCache        = NewKafkaCache(1_000)
	MaxKafkaChanSize  = 10_000
	MaxKafkaCacheSize = 1_000
)

func ListenTxKafkaProducer(
	ctx context.Context,
	txKafkaProducer *kafka.KafkaProducer,
	config ethconfig.XLayerConfig,
	logger log.Logger,
	blockInfoChan chan *realtimeTypes.BlockInfo,
	txInfoChan chan *state.TxInfo) {
	if !sequencer.IsSequencer() {
		logger.Info("TxKafkaProducer is disabled on non-sequencer, skipping")
		return
	}

	if !config.Kafka.Enable {
		logger.Info("Tx Kafka is disabled, skipping")
		return
	}

	for {
		var err error
		currHeight := uint64(0)

		select {
		case <-ctx.Done():
			return
		case blockInfo := <-blockInfoChan:
			currHeight = blockInfo.Header.Number.Uint64()
			err = txKafkaProducer.SendKafkaBlockInfo(ctx, blockInfo.Header, blockInfo.TxCount)
		case txInfo := <-txInfoChan:
			currHeight = txInfo.BlockNumber
			if currHeight <= 1 {
				continue
			}
			changeset := state.CollectChangeset(txInfo.Entries)
			err = txKafkaProducer.SendKafkaTransaction(ctx, txInfo.BlockNumber, txInfo.Tx, txInfo.Receipt, txInfo.InnerTxs, changeset)
		}

		if err != nil {
			logger.Error("Failed to send kafka message, trigger error message", "error", err, "currHeight", currHeight)
			err = txKafkaProducer.SendKafkaErrorTrigger(ctx, currHeight)
			if err != nil {
				logger.Error("Failed to send error trigger message", "error", err, "blockNumber", currHeight)
			}
			continue
		}
	}
}

func ListenTxKafkaConsumer(
	ctx context.Context,
	txKafkaConsumer *kafka.KafkaConsumer,
	config ethconfig.XLayerConfig,
	logger log.Logger,
	realtimeCache *RealtimeCache,
	finishChan chan uint64) {
	if sequencer.IsSequencer() {
		logger.Info("TxKafkaConsumer is disabled on sequencer, skipping")
		return
	}

	if !config.Kafka.Enable {
		logger.Info("Tx Kafka is disabled, skipping")
		return
	}

	errorFlag.Store(false)
	blockMsgsChan := make(chan kafkaTypes.BlockMessage, MaxKafkaChanSize)
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, MaxKafkaChanSize)
	errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, MaxKafkaChanSize)
	errorChan := make(chan error, 1)

	// Start the kafka consumer
	go txKafkaConsumer.ConsumeKafka(ctx, blockMsgsChan, txMsgsChan, errorMsgsChan, errorChan, logger)

	// Start realtime loop
	go RealtimeLoop(ctx, logger, realtimeCache)

	for {
		select {
		case <-ctx.Done():
			return
		case finishHeight := <-finishChan:
			realtimeCache.PutExecutionHeight(finishHeight)
			logger.Info("Received finish signal from execution", "finishHeight", finishHeight)
		case blockMsg := <-blockMsgsChan:
			header, _, err := blockMsg.GetBlockInfo()
			if err != nil {
				logger.Error("Failed to consume block message from kafka", "error", err)
				continue
			}
			if header.Number.Uint64() <= realtimeCache.GetHighestConfirmHeight() {
				// Ignore block msgs from previous blocks
				logger.Info("Ignoring block message from previous block", "blockMsg", blockMsg)
				continue
			}
			kafkaCache.BlockMsgCache.Add(&blockMsg)
			logger.Info("Received block message", "blockMsg", blockMsg)
		case txMsg := <-txMsgsChan:
			if err := txMsg.Validate(); err != nil {
				logger.Error("Failed to consume transaction message from kafka", "error", err)
				continue
			}
			if txMsg.BlockNumber <= realtimeCache.GetHighestConfirmHeight() {
				// Ignore txs from previous blocks
				logger.Info("Ignoring transaction message from previous block", "txMsg", txMsg)
				continue
			}
			kafkaCache.TxMsgCache.Add(&txMsg)
			logger.Info("Received transaction message", "txMsg", txMsg)
		case errorTriggerMsg := <-errorMsgsChan:
			resetFlag.Store(true)
			triggerHeight := errorTriggerMsg.BlockNumber
			logger.Info("Received error trigger message, flushing realtime cache", "triggerHeight", triggerHeight)
		case err := <-errorChan:
			errorFlag.Store(true)
			logger.Error("Kafka consumer failed", "error", err)
			return
		}
	}
}

func RealtimeLoop(ctx context.Context, logger log.Logger, realtimeCache *RealtimeCache) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check for kafka error
		if errorFlag.Load() {
			logger.Error("Kafka error, stopping realtime loop")
			return
		}

		// Check for reset trigger
		if resetFlag.Load() {
			resetRealtimeCache(realtimeCache)
			continue
		}

		// Check if realtime cache is ready
		if !readyFlag.Load() {
			if ok := tryInitRealtimeCache(realtimeCache, logger); !ok {
				time.Sleep(1 * time.Second)
			}
			continue
		}

		// Check for corrupted cache
		lastConfirmHeight := realtimeCache.GetHighestConfirmHeight()
		lastExecutionHeight := realtimeCache.GetExecutionHeight()
		if lastConfirmHeight != 0 && lastConfirmHeight < lastExecutionHeight {
			// Execution is ahead of cache. This should not happen
			resetFlag.Store(true)
			logger.Error("Execution height is ahead of cache confirm height", "lastConfirmHeight", lastConfirmHeight, "lastExecutionHeight", lastExecutionHeight)
			continue
		}

		// Sync state cache with kafka data
		pendingHeight := realtimeCache.GetHighestPendingHeight()
		lowestKafkaHeight := kafkaCache.GetLowestBlockHeight()
		if lowestKafkaHeight != 0 {
			// New block msg to process. Enforce that header msgs are received in order
			nextHeight := pendingHeight + 1
			if pendingHeight == 0 {
				// First block msg after cache init
				nextHeight = realtimeCache.State.GetSnapshotHeight() + 1
			}

			// Get next block msg and tx msgs
			blockMsg, ok := kafkaCache.BlockMsgCache.Pop(nextHeight)
			if ok {
				// Try close the previous block
				realtimeCache.TryCloseBlockFromBlockMsg(pendingHeight, blockMsg)

				// Process block msg
				err := realtimeCache.TryApplyBlockMsg(nextHeight, blockMsg)
				if err != nil {
					// Apply state error. Reset cache
					resetFlag.Store(true)
					logger.Error("Failed to apply block msg and tx msgs", "error", err, "nextHeight", nextHeight)
				}
				realtimeCache.PutHighestPendingHeight(nextHeight)

				// Flush block msg cache
				kafkaCache.BlockMsgCache.Flush(nextHeight)
			}
		}

		// Handle pending blocks
		err := realtimeCache.HandlePendingBlocks(kafkaCache)
		if err != nil {
			// Handle pending blocks error. Reset cache
			resetFlag.Store(true)
			logger.Error("Handle pending blocks failed", "error", err)
		}
	}
}

// tryInitRealtimeCache checks if the realtime cache can be initialized by comparing
// the current execution height with the lowest kafka cache height.
func tryInitRealtimeCache(realtimeCache *RealtimeCache, logger log.Logger) bool {
	executionHeight := realtimeCache.GetExecutionHeight()
	lowestKafkaHeight := kafkaCache.GetLowestBlockHeight()
	if executionHeight == 0 || lowestKafkaHeight == 0 {
		// No kafka message or rpc execution. Skip init
		return false
	}

	realtimeCache.Clear()
	err := realtimeCache.State.InitSnapshotReader()
	if err != nil {
		logger.Error("Failed to initialize snapshot reader", "error", err)
		return false
	}

	snapshotHeight := realtimeCache.State.GetSnapshotHeight()
	if lowestKafkaHeight <= snapshotHeight {
		readyFlag.Store(true)
		logger.Info("Realtime cache initialized")

		// Flush all kafka data less than or equal to snapshot reader height
		kafkaCache.Flush(snapshotHeight)

		return true
	}

	// The current snapshot reader height is behind kafka cache height. We will wait for the execution
	// height to catch up to kafka cache height before re-initializing the snapshot reader.
	logger.Info("Init realtime cache failed, waiting for execution height to catch up to kafka cache height", "lowestKafkaHeight", lowestKafkaHeight, "executionHeight", executionHeight, "snapshotHeight", snapshotHeight)
	return false
}

// resetRealtimeCache clears the realtime cache and resets the state flags
func resetRealtimeCache(realtimeCache *RealtimeCache) {
	realtimeCache.Clear()
	resetFlag.Store(false)
	readyFlag.Store(false)
}
