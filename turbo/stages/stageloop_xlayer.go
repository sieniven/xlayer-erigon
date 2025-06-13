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
	statelessCache *zktypes.StatelessCache,
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
			statelessCache.PutHeader(header.Number.Uint64(), header)
			statelessCache.PutTxCount(header.Number.Uint64()-1, int64(prevBlockTxCount))

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
			statelessCache.PutTxInfo(blockNumber, tx.Hash(), tx, receipt, innerTxs)

			// 2. Check state data
			changeset, err := txMsg.GetChangeset()
			if err != nil {
				logger.Error("Failed to consume tx changeset message from kafka", "error", err)
				continue
			}

			deliverTxChan <- txMsg

			logger.Info("Received transaction message", "tx", tx, "blockNumber", blockNumber, "receipt", receipt, "innerTxs", innerTxs, "changeset", changeset)
			for _, log := range receipt.Logs {
				logger.Info("    log message", "log", log)
			}
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
	stateCache *state.PlainStateCache,
	statelessCache *zktypes.StatelessCache) {
	if sequencer.IsSequencer() {
		logger.Info("HandleTxKafkaMessage is disabled on sequencer, skipping")
		return
	}

	tx, err := db.BeginRo(ctx)
	if err != nil {
		logger.Error("Failed to begin db tranasaction", "err", err)
		return
	}

	stateCache.Reset(tx)

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
			lastIncomplete := statelessCache.GetLastIncomplete()

			if lastIncomplete <= finishHeight {
				// Reset state cache
				stateCache.Reset(tx)
				stateCache.UpdateReady(false)
				nextTxIndex = 0
				statelessCache.MarkCompleted(finishHeight)
				continue
			}

			if stateCache.IsReady() {
				continue
			}

			if finishHeight >= stateCache.TxBlockNumber() {
				stateCache.UpdateReady(true)
			}
		case msg := <-deliverBlockInfoChan:
			logger.Info("Fetched a blockInfo message", "blockNumber", msg.Header.Number.Uint64(), "lastFinishHeight", lastFinishHeight)
			lastIncomplete := statelessCache.GetLastIncomplete()

			if lastIncomplete == msg.Header.Number.Uint64()-1 && (msg.PrevBlockTxCount == int64(nextTxIndex) || msg.PrevBlockTxCount == int64(0)) {
				nextTxIndex = 0
				statelessCache.MarkCompleted(lastIncomplete)
				lastIncomplete = skipEmptyBlock(statelessCache, lastIncomplete+1)
				nextTxIndex, err := handlePending(stateCache, &pendingTxMsgs, statelessCache, lastIncomplete, nextTxIndex, false)
				if err != nil {
					logger.Error("Failed to apply pending tx changeset to state cache", "nextTxIndex", nextTxIndex, "error", err)
				}
			}

		case msg := <-deliverTxChan:
			logger.Info("Fetched a transaction message", "blockNumber", msg.BlockNumber, "txIndex", msg.Receipt.TransactionIndex, "lastFinishHeight", lastFinishHeight)
			if msg.BlockNumber <= lastFinishHeight {
				// Discard this stale transaction message
				continue
			}

			lastIncomplete := statelessCache.GetLastIncomplete()
			logger.Info("GetLastIncomplete", "lastIncomplete", lastIncomplete)
			if msg.BlockNumber < lastIncomplete {
				// Discard this stale transaction message
				continue
			} else if msg.BlockNumber == lastIncomplete {
				// Check if the transaction index is matched
				logger.Info("Check tx index", "msg.TxIndex", msg.Receipt.TransactionIndex, "nextTxIndex", nextTxIndex)
				if msg.Receipt.TransactionIndex == uint(nextTxIndex) {
					if err := stateCache.ApplyChangeset(msg.Changeset, msg.BlockNumber, msg.Receipt.TransactionIndex); err != nil {
						// TODO：need to record the invalid changeset and apply it again later?
						logger.Error("Failed to apply tx changeset to state cache", "error", err)
						continue
					}
					nextTxIndex++

					_, txCount, exist := (*statelessCache).GetHeader(lastIncomplete)
					logger.Info("Get blockInfo", "lastIncomplete", lastIncomplete, "txCount", txCount, "nextTxIndex", nextTxIndex, "exist", exist)
					if exist && txCount >= 0 {
						// Check if the transaction is the last one of corresponding block
						if txCount == int64(nextTxIndex) {
							nextTxIndex = 0
							statelessCache.MarkCompleted(lastIncomplete)
							lastIncomplete = skipEmptyBlock(statelessCache, lastIncomplete+1)
							nextTxIndex, err = handlePending(stateCache, &pendingTxMsgs, statelessCache, lastIncomplete, nextTxIndex, false)
							if err != nil {
								logger.Error("Failed to apply pending tx changeset to state cache", "nextTxIndex", nextTxIndex, "error", err)
							}
							logger.Info("After handle pending", "len", pendingTxMsgs.Len())
						}
					} else {
						nextTxIndex, err = handlePending(stateCache, &pendingTxMsgs, statelessCache, lastIncomplete, nextTxIndex, true)
						if err != nil {
							logger.Error("Failed to apply pending tx changeset to state cache", "nextTxIndex", nextTxIndex, "error", err)
						}
						logger.Info("After handle pending", "len", pendingTxMsgs.Len())
					}
				} else if msg.Receipt.TransactionIndex > uint(nextTxIndex) {
					addPendingTx(&pendingTxMsgs, &msg)
					logger.Info("After add tx to pending", "len", pendingTxMsgs.Len())
				} else {
					// discard this stale transaction message
					continue
				}
			} else {
				_, txCount, exist := statelessCache.GetHeader(lastIncomplete)
				logger.Info("Get blockInfo", "lastIncomplete", lastIncomplete, "txCount", txCount, "nextTxIndex", nextTxIndex, "exist", exist)
				if exist && (txCount == 0 || (txCount > 0 && txCount == int64(nextTxIndex))) {
					statelessCache.MarkCompleted(lastIncomplete)
					lastIncomplete = skipEmptyBlock(statelessCache, lastIncomplete+1)
					nextTxIndex = 0

					logger.Info("get blockInfo", "height", lastIncomplete, "msgHeight", msg.BlockNumber, "txIndex", msg.Receipt.TransactionIndex)
					if lastIncomplete == msg.BlockNumber && msg.Receipt.TransactionIndex == 0 {
						if err := stateCache.ApplyChangeset(msg.Changeset, msg.BlockNumber, msg.Receipt.TransactionIndex); err != nil {
							// TODO：need to record the invalid changeset and apply it again later?
							logger.Error("Failed to apply tx changeset to state cache", "error", err)
							continue
						}
						nextTxIndex++

						nextTxIndex, err = handlePending(stateCache, &pendingTxMsgs, statelessCache, lastIncomplete, nextTxIndex, false)
						if err != nil {
							logger.Error("Failed to apply pending tx changeset to state cache", "nextTxIndex", nextTxIndex, "error", err)
						}
						logger.Info("After add tx to pending", "len", pendingTxMsgs.Len())
					} else {
						addPendingTx(&pendingTxMsgs, &msg)
						logger.Info("After add tx to pending", "len", pendingTxMsgs.Len())
					}
				} else {
					addPendingTx(&pendingTxMsgs, &msg)
					logger.Info("After add tx to pending", "len", pendingTxMsgs.Len())
				}
			}
		}

		logger.Info("Get the latest state value", "lastFinishHeight", lastFinishHeight, "nextTxIndex", nextTxIndex, "lastIncomplete", statelessCache.GetLastIncomplete(), "pendingLen", pendingTxMsgs.Len())
	}
}

func handlePending(stateCache *state.PlainStateCache, pendingTxMsgs *kafkaTypes.TransactionMessageSlice, statelessCache *zktypes.StatelessCache, curHeight, nextTxIndex uint64, heightLock bool) (uint64, error) {
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
				if err := stateCache.ApplyChangeset(msg.Changeset, msg.BlockNumber, msg.Receipt.TransactionIndex); err != nil {
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

			_, txCount, exist := statelessCache.GetHeader(curHeight)
			// Not sure how many transactions there are in curHeight block
			if !exist || txCount < 0 {
				heightLock = true
				handled--
				continue
			}

			// Apply changeset and also skip empty block if it is necessary
			if msg.Receipt.TransactionIndex == uint(nextTxIndex) {
				if err := stateCache.ApplyChangeset(msg.Changeset, msg.BlockNumber, msg.Receipt.TransactionIndex); err != nil {
					*pendingTxMsgs = (*pendingTxMsgs)[handled:]
					return nextTxIndex, err
				}
				nextTxIndex++

				if txCount == int64(nextTxIndex) {
					statelessCache.MarkCompleted(curHeight)
					curHeight = skipEmptyBlock(statelessCache, curHeight+1)
					nextTxIndex = 0
				}
			}
		}
	}

	*pendingTxMsgs = (*pendingTxMsgs)[handled:]
	return nextTxIndex, nil
}

func skipEmptyBlock(statelessCache *zktypes.StatelessCache, start uint64) uint64 {
	for {
		_, txCount, exist := statelessCache.GetHeader(start)
		if !exist || txCount != 0 {
			return start
		}
		statelessCache.MarkCompleted(start)
		start++
	}
}

func addPendingTx(pendingTxMsgs *kafkaTypes.TransactionMessageSlice, msg *kafkaTypes.TransactionMessage) {
	*pendingTxMsgs = append(*pendingTxMsgs, msg)
	sort.Sort(pendingTxMsgs)
}
