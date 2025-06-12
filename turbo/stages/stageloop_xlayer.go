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

func ListenTxKafkaConsumer(
	ctx context.Context,
	txKafkaConsumer *kafka.KafkaConsumer,
	config ethconfig.XLayerConfig,
	logger log.Logger,
	txInfoMap *zktypes.TxInfoMap,
	blockInfoMap *zktypes.BlockInfoMap,
	deliverTxChan chan kafkaTypes.TransactionMessage,
	deliverBlockInfoChan chan kafkaTypes.BlockMessage) {
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
			blockInfoMap.PutTxCount(header.Number.Uint64()-1, int64(prevBlockTxCount))

			deliverBlockInfoChan <- blockMsg

			logger.Info("Received block message", "header", header.Number, "prevBlockTxCount", prevBlockTxCount)
		case txMsg := <-txMsgsChan:
			// 1. Check non-state data
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

			// 2. Check state data
			changeset, err := txMsg.GetChangeset()
			if err != nil {
				logger.Error("Failed to consume tx changeset message from kafka", "error", err)
				continue
			}

			deliverTxChan <- txMsg

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
			if currHeight <= 1 {
				continue
			}

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
	deliverTxChan chan kafkaTypes.TransactionMessage,
	deliverBlockInfoChan chan kafkaTypes.BlockMessage,
	finishChan chan uint64,
	blockInfoMap *zktypes.BlockInfoMap) {
	if sequencer.IsSequencer() {
		logger.Info("HandleTxKafkaMessage is disabled on sequencer, skipping")
		return
	}

	tx, err := db.BeginRo(ctx)
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

	stateCache := state.NewPlainStateCache(tx)

	defer func() {
		if err := dsClient.Stop(); err != nil {
			logger.Error("problem stopping datastream client looking up latest ds l2 block", "err", err)
		}
	}()

	var (
		lastFinishHeight = uint64(0)
		nextTxIndex      = uint64(0)
		pendingTxMsgs    = kafkaTypes.TransactionMessageSlice{}
	)

	for {
		select {
		case <-ctx.Done():
			return
		case finishHeight := <-finishChan:
			logger.Info("Fetched a finish signal", "finishHeight", finishHeight)
			lastFinishHeight = finishHeight
			lastIncomplete := blockInfoMap.GetLastIncomplete(0, lastFinishHeight)

			if lastIncomplete <= finishHeight {
				// reset state cache
				stateCache = state.NewPlainStateCache(tx)
				stateCache.UpdateReady(false)
				continue
			}

			if stateCache.IsReady() {
				continue
			}

			highestHashableL2BlockNo, err := stages.GetStageProgress(tx, stages.HighestHashableL2BlockNo)
			if err != nil {
				logger.Error("Failed to get stage progress", "topic", stages.HighestHashableL2BlockNo, "error", err)
				continue
			}
			currentBlockNo := min(fullBlock.BatchNumber, highestHashableL2BlockNo)

			// TODO: How to determine whether to update ready to true except when entering here for the first time？
			if fullBlock.L2BlockNumber >= currentBlockNo {
				stateCache.UpdateReady(true)
			}
		case msg := <-deliverBlockInfoChan:
			logger.Info("Fetched a blockInfo message", "blockNumber", msg.Header.Number.Uint64()-1, lastFinishHeight)
			if !blockInfoMap.IsCompleted(msg.Header.Number.Uint64() - 1) {
				lastIncomplete := blockInfoMap.GetLastIncomplete(msg.Header.Number.Uint64()-1, lastFinishHeight)

				if lastIncomplete == msg.Header.Number.Uint64()-1 && msg.PrevBlockTxCount == int64(nextTxIndex) {
					nextTxIndex = 0
					blockInfoMap.MarkCompleted(lastIncomplete)
					lastIncomplete = skipEmptyBlock(blockInfoMap, lastIncomplete)
					_, err := handlePending(stateCache, &pendingTxMsgs, blockInfoMap, lastIncomplete, nextTxIndex, false)
					if err != nil {
						logger.Error("Failed to apply pending tx changeset to state cache", "nextTxIndex", nextTxIndex, "error", err)
					}
				}
			}
		case msg := <-deliverTxChan:
			logger.Info("Fetched a transaction message", "blockNumber", msg.BlockNumber, "txIndex", msg.Receipt.TransactionIndex, "lastFinishHeight", lastFinishHeight)
			if msg.BlockNumber <= lastFinishHeight {
				// discard this stale transaction message
				continue
			}

			lastIncomplete := blockInfoMap.GetLastIncomplete(msg.BlockNumber, lastFinishHeight)
			logger.Info("GetLastIncomplete", "lastIncomplete", lastIncomplete)
			if msg.BlockNumber < lastIncomplete {
				// discard this stale transaction message
				continue
			} else if msg.BlockNumber == lastIncomplete {
				// check if the transaction index is matched
				logger.Info("check tx index", "msg.TxIndex", msg.Receipt.TransactionIndex, "nextTxIndex", nextTxIndex)
				if msg.Receipt.TransactionIndex == uint(nextTxIndex) {
					if err := stateCache.ApplyChangeset(msg.Changeset); err != nil {
						// TODO：need to record the invalid changeset and apply it again later?
						logger.Error("Failed to apply tx changeset to state cache", "error", err)
						continue
					}
					nextTxIndex++

					_, txCount, exist := (*blockInfoMap).Get(lastIncomplete)
					logger.Info("get blockInfo", "height", lastIncomplete, "txCount", txCount, "exist", exist)
					if exist && txCount >= 0 {
						// check if the transaction is the last one of corresponding block
						if txCount == int64(nextTxIndex) {
							nextTxIndex = 0
							blockInfoMap.MarkCompleted(lastIncomplete)
							lastIncomplete = skipEmptyBlock(blockInfoMap, lastIncomplete+1)
							nextTxIndex, err = handlePending(stateCache, &pendingTxMsgs, blockInfoMap, lastIncomplete, nextTxIndex, false)
							if err != nil {
								logger.Error("Failed to apply pending tx changeset to state cache", "nextTxIndex", nextTxIndex, "error", err)
							}
						}
					} else {
						nextTxIndex, err = handlePending(stateCache, &pendingTxMsgs, blockInfoMap, lastIncomplete, nextTxIndex, true)
						if err != nil {
							logger.Error("Failed to apply pending tx changeset to state cache", "nextTxIndex", nextTxIndex, "error", err)
						}
					}
				} else {
					addPendingTx(&pendingTxMsgs, &msg)
					logger.Info("Pending length", "len", pendingTxMsgs.Len())
				}
			} else {
				_, txCount, exist := blockInfoMap.Get(lastIncomplete)
				logger.Info("XXX", "txCount", txCount, "nextTxIndex", nextTxIndex, "exist", exist)
				if exist && (txCount == 0 || (txCount > 0 && txCount == int64(nextTxIndex))) {
					blockInfoMap.MarkCompleted(lastIncomplete)
					lastIncomplete = skipEmptyBlock(blockInfoMap, lastIncomplete)

					if lastIncomplete == msg.BlockNumber && msg.Receipt.TransactionIndex == 0 {
						if err := stateCache.ApplyChangeset(msg.Changeset); err != nil {
							// TODO：need to record the invalid changeset and apply it again later?
							logger.Error("Failed to apply tx changeset to state cache", "error", err)
							continue
						}
						nextTxIndex++

						nextTxIndex, err = handlePending(stateCache, &pendingTxMsgs, blockInfoMap, lastIncomplete, nextTxIndex, false)
						if err != nil {
							logger.Error("Failed to apply pending tx changeset to state cache", "nextTxIndex", nextTxIndex, "error", err)
						}
					} else {
						addPendingTx(&pendingTxMsgs, &msg)
					}
				} else {
					addPendingTx(&pendingTxMsgs, &msg)
					logger.Info("Pending length", "len", pendingTxMsgs.Len())
				}
			}
		}
	}
}

func handlePending(stateCache *state.PlainStateCache, pendingTxMsgs *kafkaTypes.TransactionMessageSlice, blockInfoMap *zktypes.BlockInfoMap, curHeight, nextTxIndex uint64, heightLock bool) (uint64, error) {
	var handled int
	for ; handled < pendingTxMsgs.Len(); handled++ {
		msg := (*pendingTxMsgs)[handled]
		height := msg.BlockNumber
		if height < curHeight {
			continue
		}

		if heightLock {
			if height != curHeight {
				break
			}

			if msg.Receipt.TransactionIndex == uint(nextTxIndex) {
				if err := stateCache.ApplyChangeset(msg.Changeset); err != nil {
					*pendingTxMsgs = (*pendingTxMsgs)[handled:]
					return nextTxIndex, err
				}
				nextTxIndex++
			} else {
				break
			}
		} else {
			// Since curHeight is non-empty block height, it must contain transactions
			if height != curHeight {
				break
			}

			_, txCount, exist := blockInfoMap.Get(curHeight)
			// Not sure how many transactions there are in curHeight block
			if !exist || txCount < 0 {
				heightLock = true
				handled--
				continue
			}

			// Apply changeset and also skip empty block if it is necessary
			if msg.Receipt.TransactionIndex == uint(nextTxIndex) {
				if err := stateCache.ApplyChangeset(msg.Changeset); err != nil {
					*pendingTxMsgs = (*pendingTxMsgs)[handled:]
					return nextTxIndex, err
				}
				nextTxIndex++

				if txCount == int64(nextTxIndex) {
					blockInfoMap.MarkCompleted(curHeight)
					curHeight = skipEmptyBlock(blockInfoMap, curHeight+1)
					nextTxIndex = 0
				}
			}
		}
	}

	*pendingTxMsgs = (*pendingTxMsgs)[handled:]
	return nextTxIndex, nil
}

func skipEmptyBlock(blockInfoMap *zktypes.BlockInfoMap, start uint64) uint64 {
	for {
		_, txCount, exist := blockInfoMap.Get(start)
		if !exist || txCount != 0 {
			return start
		}
		blockInfoMap.MarkCompleted(start)
		start++
	}
}

func addPendingTx(pendingTxMsgs *kafkaTypes.TransactionMessageSlice, msg *kafkaTypes.TransactionMessage) {
	*pendingTxMsgs = append(*pendingTxMsgs, msg)
	sort.Sort(pendingTxMsgs)
}
