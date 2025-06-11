package stages

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/membatch"

	"github.com/ledgerwatch/log/v3"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/zk/datastream/client"
	"github.com/ledgerwatch/erigon/zk/kafka"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/kafka/types"
	"github.com/ledgerwatch/erigon/zk/sequencer"

	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/eth/stagedsync"
	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
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
		logger.Error("Failed to flush data to DB", "error", err)
		return
	}
	cache.TruncateSmtCacheList(saveData.BlockHeight)
}

func ListenTxKafkaConsumer(ctx context.Context, txKafkaConsumer *kafka.KafkaConsumer, config ethconfig.XLayerConfig, logger log.Logger, txInfoMap *zktypes.TxInfoMap, blockInfoMap *zktypes.BlockInfoMap, stateCache *state.PlainStateCache) {
	if sequencer.IsSequencer() {
		logger.Info("TxKafkaConsumer is disabled on sequencer, skipping")
		return
	}

	if !config.Kafka.Enable {
		logger.Info("Tx Kafka is disabled, skipping")
		return
	}

	// Start the kafka consumer
	blockMsgsChan := make(chan kafkaTypes.BlockMessage, MaxKafkaChanSize)
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, MaxKafkaChanSize)
	errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, MaxKafkaChanSize)
	errorChan := make(chan error, 1)
	go txKafkaConsumer.ConsumeKafka(ctx, blockMsgsChan, txMsgsChan, errorMsgsChan, errorChan, logger)

	// TODO: Start snapshot and sync height with incoming kafka messages

	for {
		select {
		case <-ctx.Done():
			return
		case blockMsg := <-blockMsgsChan:
			header, prevBlockTxCount, err := blockMsg.GetBlockInfo()
			if err != nil {
				logger.Error("Failed to consume block message from kafka", "error", err)
				continue
			}
			blockInfoMap.PutHeader(header.Number.Uint64(), header)
			blockInfoMap.PutTxCount(header.Number.Uint64()-1, prevBlockTxCount)
			logger.Info("Received block message", "header", header, "prevBlockTxCount", prevBlockTxCount)
		case txMsg := <-txMsgsChan:
			// 1. Process non-state data
			tx, blockNumber, err := txMsg.GetTransaction()
			if err != nil {
				logger.Error("Failed to consume transaction message from kafka", "error", err)
				continue
			}
			receipt, err := txMsg.GetReceipt()
			if err != nil {
				logger.Error("Failed to consume tx receipt message from kafka", "error", err)
				continue
			}
			innerTxs, err := txMsg.GetInnerTxs()
			if err != nil {
				logger.Error("Failed to consume tx innerTxs message from kafka", "error", err)
				continue
			}
			txInfoMap.Put(tx.Hash(), tx, receipt, innerTxs)

			// 2. Process state data
			changeset, err := txMsg.GetChangeset()
			if err != nil {
				logger.Error("Failed to consume tx changeset message from kafka", "error", err)
				continue
			}
			stateCache.ApplyChangeset(changeset)

			logger.Info("Received transaction message", "tx", tx, "blockNumber", blockNumber, "receipt", receipt, "innerTxs", innerTxs, "changeset", changeset)
		case errorTriggerMsg := <-errorMsgsChan:
			triggerHeight := errorTriggerMsg.BlockNumber
			logger.Info("Received error trigger message", "triggerHeight", triggerHeight)

			// TODO: handle trigger unwind here on producer side error

		case err := <-errorChan:
			logger.Error("Kafka consumer Failed", "error", err)
			return
		}
	}
}

func ListenTxKafkaProducer(
	ctx context.Context,
	txKafkaProducer *kafka.KafkaProducer,
	config ethconfig.XLayerConfig,
	logger log.Logger,
	blockInfoChan chan *zktypes.BlockInfo,
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
			// log.Info("Kafka prepare to send blockInfo", "blockInfo", blockInfo)
			err = txKafkaProducer.SendKafkaBlockInfo(ctx, blockInfo.Header, blockInfo.TxCount)
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

func HandleTxKafkaMessage(
	ctx context.Context,
	db kv.RwDB,
	ethCfg *ethconfig.Config,
	logger log.Logger,
	deliverChan chan *kafkaTypes.TransactionMessage,
	finishChan chan struct{},
	stateCache *state.PlainStateCache,
	blockInfoMap *zktypes.BlockInfoMap) {
	if sequencer.IsSequencer() {
		logger.Info("HandleTxKafkaMessage is disabled on sequencer, skipping")
		return
	}

	tx, err := db.BeginRw(ctx)
	if err != nil {
		logger.Error("Failed to begin db tranasaction", "err", err)
		return
	}

	latestForkId, err := stages.GetStageProgress(tx, stages.ForkId)
	if err != nil {
		logger.Error("Failed to get stage progress of forkid", "err", err)
		return
	}
	dsClient := client.NewClient(ctx, ethCfg.L2DataStreamerUrl, ethCfg.L2DataStreamerUseTLS, ethCfg.DatastreamVersion, ethCfg.L2DataStreamerTimeout, uint16(latestForkId))
	if err = dsClient.Start(); err != nil {
		logger.Error("Failed to start dsClient", "err", err)
		return
	}

	fullBlock, err := dsClient.GetLatestL2Block()
	if err != nil {
		logger.Error("Failed to get latest L2Block", "err", err)
		return
	}

	defer func() {
		if err := dsClient.Stop(); err != nil {
			logger.Error("problem stopping datastream client looking up latest ds l2 block", "err", err)
		}
	}()

	var (
		lastHeight    = uint64(0)
		nextTxIndex   = uint64(0)
		pendingTxMsgs = kafkaTypes.TransactionMessageSlice{}
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-finishChan:
			logger.Info("Fetched a finish signal")
			if stateCache.IsReady() {
				continue
			}
			highestHashableL2BlockNo, err := stages.GetStageProgress(tx, stages.HighestHashableL2BlockNo)
			if err != nil {
				logger.Error("Failed to get stage progress", "topic", stages.HighestHashableL2BlockNo, "error", err)
				continue
			}
			currentBlockNo := min(fullBlock.BatchNumber, highestHashableL2BlockNo)

			if fullBlock.L2BlockNumber >= currentBlockNo {
				stateCache.UpdateReady(true)
			}
		case msg := <-deliverChan:
			_, prevBlockTxCount, exist := blockInfoMap.Get(msg.BlockNumber)
			if exist && msg.BlockNumber == lastHeight || (lastHeight == 0 && msg.Receipt.TransactionIndex == 0) {
				lastHeight = msg.BlockNumber

				if msg.Receipt.TransactionIndex == uint(prevBlockTxCount) {
					if err := stateCache.ApplyChangeset(msg.Changeset); err != nil {
						// TODO：need to record the invalid changeset and apply it again later?
						logger.Error("Failed to apply tx changeset to state cache", "error", err)
						continue
					}

					lastHeight, nextTxIndex, err = handlePending(stateCache, &pendingTxMsgs, blockInfoMap, lastHeight, nextTxIndex)
					if err != nil {
						logger.Error("Failed to apply pending tx changeset to state cache", "lastHeight", lastHeight, "nextTxIndex", nextTxIndex, "error", err)
					}
				} else {
					pendingTxMsgs = append(pendingTxMsgs, msg)
					sort.Sort(pendingTxMsgs)

					// TODO: if pending msgs length is too large, should ask kafka for the missing tx msgs

				}
			} else if msg.BlockNumber == lastHeight+1 && msg.Receipt.TransactionIndex == 0 && nextTxIndex == prevBlockTxCount+1 {
				// the last handled msg is the last one of lastHeight, and the new msg is the first one of next block
				if err := stateCache.ApplyChangeset(msg.Changeset); err != nil {
					logger.Error("Failed to apply tx changeset to state cache", "error", err)
					continue
				}

				lastHeight, nextTxIndex, err = handlePending(stateCache, &pendingTxMsgs, blockInfoMap, lastHeight, nextTxIndex)
				if err != nil {
					logger.Error("Failed to apply pending tx changeset to state cache", "lastHeight", lastHeight, "nextTxIndex", nextTxIndex, "error", err)
				}

				if err := stateCache.ApplyChangeset(msg.Changeset); err != nil {
					logger.Error("Failed to apply pending tx changeset to state cache", "lastHeight", lastHeight, "nextTxIndex", nextTxIndex, "error", err)
				}
			} else {
				if msg.BlockNumber < lastHeight {
					logger.Warn("Got a stale transaction message, discarded it")
					continue
				}

				pendingTxMsgs = append(pendingTxMsgs, msg)
				sort.Sort(pendingTxMsgs)
			}
		}
	}
}

func handlePending(stateCache *state.PlainStateCache, pendingTxMsgs *kafkaTypes.TransactionMessageSlice, blockInfoMap *zktypes.BlockInfoMap, lastHeight, nextTxIndex uint64) (uint64, uint64, error) {
	handled, newLastHeight, newNextTxIndex := 0, lastHeight, nextTxIndex
	for ; handled < len(*pendingTxMsgs); handled++ {
		_, prevBlockTxCount, exist := blockInfoMap.Get((*pendingTxMsgs)[handled].BlockNumber)
		if !exist {
			return newLastHeight, newNextTxIndex, nil
		}

		if (*pendingTxMsgs)[handled].BlockNumber == newLastHeight && (*pendingTxMsgs)[handled].Receipt.TransactionIndex == uint(prevBlockTxCount) {
			if err := stateCache.ApplyChangeset((*pendingTxMsgs)[handled].Changeset); err != nil {
				return newLastHeight, newNextTxIndex, err
			}
			newLastHeight = (*pendingTxMsgs)[handled].BlockNumber
			newNextTxIndex++
		} else if (*pendingTxMsgs)[handled].BlockNumber == newLastHeight+1 && (*pendingTxMsgs)[handled].Receipt.TransactionIndex == 0 && nextTxIndex == prevBlockTxCount+1 {
			if err := stateCache.ApplyChangeset((*pendingTxMsgs)[handled].Changeset); err != nil {
				return newLastHeight, newNextTxIndex, err
			}
			newLastHeight = (*pendingTxMsgs)[handled].BlockNumber
			newNextTxIndex = 1
		}
	}
	*pendingTxMsgs = (*pendingTxMsgs)[handled:]
	return newLastHeight, newNextTxIndex, nil
}
