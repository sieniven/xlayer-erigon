package stages

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/membatch"

	"github.com/ledgerwatch/log/v3"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/zk/kafka"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/kafka/types"
	"github.com/ledgerwatch/erigon/zk/sequencer"

	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/eth/stagedsync"
	"github.com/ledgerwatch/erigon/zk/smt"
	zkStages "github.com/ledgerwatch/erigon/zk/stages"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
)

const (
	MaxKafkaChanSize = 10_000
)

func AsyncFlushSmtData(ctx context.Context,
	_db kv.RwDB,
	s *stagedsync.Sync,
	config ethconfig.XLayerConfig,
	logger log.Logger,
	smtFlushDoneCh chan struct{},
) {
	if !sequencer.IsSequencer() || !config.EnableAsyncCommit {
		logger.Info("AsyncFlushSmtData skipped",
			"isSequencer", sequencer.IsSequencer(),
			"enableAsyncCommit", config.EnableAsyncCommit)
		return
	}

	db, ok := _db.(*mdbx.MdbxKV)
	if !ok {
		logger.Error("invalid database type, expected *mdbx.MdbxKV", "type", fmt.Sprintf("%T", _db))
		return
	}

	cache := s.GetCache()
	const maxWorkers = 6
	workerPool := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	defer func() {
		logger.Info("Waiting for all flush operations to complete...")
		wg.Wait()
		logger.Info("All flush operations completed, exiting AsyncFlushSmtData")
		smtFlushDoneCh <- struct{}{}
	}()

	if config.SequencerReplay {
		replayDone, _ := zkStages.WaitResequenceBatchDone()
		go func() {
			<-replayDone
			logger.Info("AsyncFlushSmtData received replay done signal")
			handleShutdown(s, config, &wg, workerPool, db, cache, logger)
			logger.Info("Waiting for all flush operations to complete...")
			wg.Wait()
			logger.Info("All flush operations completed, exiting AsyncFlushSmtData")
			os.Exit(0)
		}()
	}

	for {
		select {
		case saveData, ok := <-cache.SmtCacheDataCh:
			if !ok {
				logger.Info("SmtCacheCh closed, stopping AsyncFlushSmtData")
				return
			}
			dispatchFlushTask(context.Background(), &wg, workerPool, db, cache, saveData, logger)

		case <-ctx.Done():
			logger.Info("AsyncFlushSmtData received stop signal", "reason", ctx.Err())
			handleShutdown(s, config, &wg, workerPool, db, cache, logger)
			return
		}
	}
}

func dispatchFlushTask(ctx context.Context, wg *sync.WaitGroup, workerPool chan struct{},
	db *mdbx.MdbxKV, cache *smt.SmtCache, saveData smt.SmtCacheSave, logger log.Logger) {
	select {
	case workerPool <- struct{}{}: // get working slot
		wg.Add(1)
		go func() {
			defer func() {
				<-workerPool // release working slot
				wg.Done()
			}()
			FlushDataToDB(ctx, db, logger, cache, saveData)
		}()

	case <-ctx.Done():
		logger.Debug("Skipped flush task due to context cancellation", "reason", ctx.Err())
	}
}

func handleShutdown(s *stagedsync.Sync, config ethconfig.XLayerConfig,
	wg *sync.WaitGroup, workerPool chan struct{}, db *mdbx.MdbxKV, cache *smt.SmtCache, logger log.Logger) {
	s.FlushSmtCache(config.StandaloneSMTDatabase, true)

	for {
		select {
		case saveData, ok := <-cache.SmtCacheDataCh:
			if !ok {
				logger.Info("SmtCacheCh closed during shutdown")
				return
			}
			dispatchFlushTask(context.Background(), wg, workerPool, db, cache, saveData, logger)
		default:
			logger.Debug("No more data in SmtCacheDataCh during shutdown")
			return
		}
	}
}

func FlushDataToDB(ctx context.Context, db *mdbx.MdbxKV, logger log.Logger, cache *smt.SmtCache, saveData smt.SmtCacheSave) {
	err := db.Batch(func(tx kv.RwTx) error {
		batch := membatch.NewHashBatch(tx, ctx.Done(), "", logger)
		defer batch.Close()
		batch.SetCache(saveData.SmtData)
		return batch.Flush(ctx, tx)
	})
	if err != nil {
		logger.Error("failed to flush data to DB", "error", err)
		return
	}
	cache.TruncateSmtCacheList(saveData.BlockHeight)
}

func ListenTxKafkaConsumer(ctx context.Context, txKafkaConsumer *kafka.KafkaConsumer, config ethconfig.XLayerConfig, logger log.Logger, txInfoMap *zktypes.TxInfoMap, headerMap *zktypes.HeaderMap, stateCache *state.PlainStateCache) {
	if sequencer.IsSequencer() {
		logger.Info("txKafkaConsumer is disabled on sequencer, skipping")
		return
	}

	if !config.Kafka.Enable {
		logger.Info("Tx Kafka is disabled, skipping")
		return
	}

	// Start the kafka consumer
	headersChan := make(chan types.Header, MaxKafkaChanSize)
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, MaxKafkaChanSize)
	errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, MaxKafkaChanSize)
	errorChan := make(chan error, 1)
	go txKafkaConsumer.ConsumeKafka(ctx, headersChan, txMsgsChan, errorMsgsChan, errorChan, logger)

	// TODO: Start snapshot and sync height with incoming kafka messages

	for {
		select {
		case <-ctx.Done():
			return
		case header := <-headersChan:
			headerMap.Put(header.Number.Uint64(), &header)
			logger.Info("Received header message", "header", header)
		case txMsg := <-txMsgsChan:
			// 1. Process non-state data
			tx, blockNumber, err := txMsg.GetTransaction()
			if err != nil {
				logger.Error("failed to consume transaction message from kafka", "error", err)
				continue
			}
			receipt, err := txMsg.GetReceipt()
			if err != nil {
				logger.Error("failed to consume tx receipt message from kafka", "error", err)
				continue
			}
			innerTxs, err := txMsg.GetInnerTxs()
			if err != nil {
				logger.Error("failed to consume tx innerTxs message from kafka", "error", err)
				continue
			}
			txInfoMap.Put(tx.Hash(), tx, receipt, innerTxs)

			// 2. Process state data
			changeset, err := txMsg.GetChangeset()
			if err != nil {
				logger.Error("failed to consume tx changeset message from kafka", "error", err)
				continue
			}
			stateCache.ApplyChangeset(changeset)

			logger.Info("Received transaction message", "tx", tx, "blockNumber", blockNumber, "receipt", receipt, "innerTxs", innerTxs, "changeset", changeset)
		case errorTriggerMsg := <-errorMsgsChan:
			triggerHeight := errorTriggerMsg.BlockNumber
			logger.Info("Received error trigger message", "triggerHeight", triggerHeight)

			// TODO: handle trigger unwind here on producer side error

		case err := <-errorChan:
			logger.Error("kafka consumer failed", "error", err)
			return
		}
	}
}

func ListenTxKafkaProducer(
	ctx context.Context,
	txKafkaProducer *kafka.KafkaProducer,
	config ethconfig.XLayerConfig,
	logger log.Logger,
	headersChan chan *types.Header,
	txInfoChan chan *state.TxInfo) {
	if !sequencer.IsSequencer() {
		logger.Info("txKafkaProducer is disabled on non-sequencer, skipping")
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
		case header := <-headersChan:
			currHeight = header.Number.Uint64()
			// log.Info("Kafka prepare to send header", "header", header)
			err = txKafkaProducer.SendKafkaBlockHeader(ctx, header)
		case txInfo := <-txInfoChan:
			currHeight = txInfo.BlockNumber
			changeset := state.CollectChangeset(txInfo.Entries)
			// log.Info("Kafka prepare to send transaction", "txhash", txInfo.Tx.Hash(), "receipt", txInfo.Receipt, "innerTxs", txInfo.InnerTxs, "changeset", changeset)
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
