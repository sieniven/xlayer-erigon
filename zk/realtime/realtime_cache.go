package realtime

import (
	"context"
	"fmt"
	"sync/atomic"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/realtime/kafka/types"
	realtimeTypes "github.com/ledgerwatch/erigon/zk/realtime/types"
)

const (
	// Kafka tx message cache size
	DefaultTxMsgSliceSize = 100

	// Stateless cache sizes
	DefaultStatelessBlockCacheSize = 1000
	DefaultStatelessTxCacheSize    = 10000

	// State cache size
	DefaultStateCacheSize = 1_000_000

	// Sync threshold config
	PendingBlocksSyncThreshold      = 3
	PendingBlocksCacheSizeThreshold = 10
)

type PendingBlockContext struct {
	// blockNum is the block number of the current pending block
	blockNum uint64
	// nextTxIndex is the next tx index to be processed in the current pending block
	nextTxIndex uint
	// txCount is the total tx count to close the current pending block. Set txCount to -1 to indicate the next block header has not been received yet.
	txCount int64
	// pendingTxs is the queue of pending txs to be processed in the current pending block
	pendingTxs *libcommon.OrderedList[*kafkaTypes.TransactionMessage]
}

type RealtimeCache struct {
	// Caches
	State     *PlainStateCache
	Stateless *realtimeTypes.StatelessCache

	// highestConfirmHeight is the highest confirmed block height closed from kafka
	highestConfirmHeight atomic.Uint64

	// highestExecutionHeight is the highest executed height on the RPC node
	highestExecutionHeight atomic.Uint64

	// highestPendingHeight is the highest pending block height received from block messages
	highestPendingHeight atomic.Uint64

	// Pending blocks cache
	pendingBlocks map[uint64]*PendingBlockContext
}

func NewRealtimeCache(ctx context.Context, db kv.RoDB) (*RealtimeCache, error) {
	stateCache, err := NewPlainStateCache(ctx, db, DefaultStateCacheSize)
	if err != nil {
		return nil, err
	}

	return &RealtimeCache{
		State:                  stateCache,
		Stateless:              realtimeTypes.NewStatelessCache(DefaultStatelessBlockCacheSize, DefaultStatelessTxCacheSize),
		highestConfirmHeight:   atomic.Uint64{},
		highestExecutionHeight: atomic.Uint64{},
		pendingBlocks:          make(map[uint64]*PendingBlockContext),
	}, nil
}

func (cache *RealtimeCache) Clear() {
	// Clear caches
	cache.Stateless.Clear()
	cache.State.Clear()

	cache.highestConfirmHeight.Store(0)
	cache.pendingBlocks = make(map[uint64]*PendingBlockContext)
}

func (cache *RealtimeCache) GetConfirmHeight() uint64 {
	return cache.highestConfirmHeight.Load()
}

func (cache *RealtimeCache) GetExecutionHeight() uint64 {
	return cache.highestExecutionHeight.Load()
}

func (cache *RealtimeCache) PutExecutionHeight(blockNum uint64) {
	if blockNum > cache.highestExecutionHeight.Load() {
		cache.highestExecutionHeight.Store(blockNum)
	}
}

func (cache *RealtimeCache) GetHighestPendingHeight() uint64 {
	return cache.highestPendingHeight.Load()
}

func (cache *RealtimeCache) PutHighestPendingHeight(blockNum uint64) {
	if blockNum > cache.highestPendingHeight.Load() {
		cache.highestPendingHeight.Store(blockNum)
	}
}

func (cache *RealtimeCache) TryCloseBlockFromBlockMsg(prevblockNum uint64, blockMsg *kafkaTypes.BlockMessage) error {
	if prevblockNum == 0 {
		// Cache init
		return nil
	}

	pendingBlockContext, ok := cache.pendingBlocks[prevblockNum]
	if !ok {
		return fmt.Errorf("prev block %d is not in pending blocks", prevblockNum)
	}
	pendingBlockContext.txCount = blockMsg.PrevBlockTxCount

	// Try close pending block
	cache.tryCloseBlock(prevblockNum, pendingBlockContext)

	return nil
}

func (cache *RealtimeCache) TryApplyBlockMsgAndTxMsgs(blockNum uint64, blockMsg *kafkaTypes.BlockMessage, sortedTxMsgs []*kafkaTypes.TransactionMessage) error {
	_, err := cache.tryCreateNewPendingBlockContext(blockNum)
	if err != nil {
		return err
	}

	cache.Stateless.PutHeader(blockNum, blockMsg.Header, blockMsg.PrevBlockTxCount)
	return cache.tryApplyBlockTxMsgs(blockNum, sortedTxMsgs)
}

func (cache *RealtimeCache) HandlePendingBlocks(kafkaCache *KafkaCache) error {
	for blockNum := range cache.pendingBlocks {
		// Check if pending block is pending for too long
		confirmHeight := cache.GetConfirmHeight()
		if confirmHeight > blockNum && confirmHeight-blockNum > PendingBlocksSyncThreshold {
			// Propagate error to reset cache
			return fmt.Errorf("block is pending for too long. Confirm height: %d, pending block num: %d", confirmHeight, blockNum)
		}

		txMsgs := kafkaCache.TxMsgCache.Pop(blockNum)
		err := cache.tryApplyBlockTxMsgs(blockNum, txMsgs)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cache *RealtimeCache) tryApplyBlockTxMsgs(blockNum uint64, sortedTxMsgs []*kafkaTypes.TransactionMessage) error {
	pendingBlockContext, ok := cache.pendingBlocks[blockNum]
	if !ok {
		// Header not received yet. Create new pending block context
		var err error
		pendingBlockContext, err = cache.tryCreateNewPendingBlockContext(blockNum)
		if err != nil {
			return err
		}
	}

	// Add to pending queue
	for _, txMsg := range sortedTxMsgs {
		pendingBlockContext.pendingTxs.Add(txMsg)
	}
	pendingBlockContext.pendingTxs.Sort()

	// Process pending txs
	processed := 0
	for _, txMsg := range pendingBlockContext.pendingTxs.Items() {
		if txMsg.Receipt.TransactionIndex != pendingBlockContext.nextTxIndex {
			break
		}

		// Apply tx msg
		tx, _, err := txMsg.GetTransaction()
		if err != nil {
			return fmt.Errorf("failed to get tx. Block number: %d, tx index: %d, error: %v", txMsg.BlockNumber, pendingBlockContext.nextTxIndex, err)
		}
		receipt, err := txMsg.GetReceipt()
		if err != nil {
			return fmt.Errorf("failed to get tx receipt. Block number: %d, tx index: %d, error: %v", txMsg.BlockNumber, pendingBlockContext.nextTxIndex, err)
		}
		innerTxs, err := txMsg.GetInnerTxs()
		if err != nil {
			return fmt.Errorf("failed to get inner txs. Block number: %d, tx index: %d, error: %v", txMsg.BlockNumber, pendingBlockContext.nextTxIndex, err)
		}
		cache.Stateless.PutTxInfo(blockNum, txMsg.Hash, tx, receipt, innerTxs)
		cache.State.ApplyChangeset(txMsg.Changeset, txMsg.BlockNumber, txMsg.Receipt.TransactionIndex)
		pendingBlockContext.nextTxIndex++
		processed++
	}

	newPendingTxs := pendingBlockContext.pendingTxs.Items()[processed:]
	pendingBlockContext.pendingTxs.SetItems(newPendingTxs)
	pendingBlockContext.pendingTxs.Sort()

	// Try to close block
	cache.tryCloseBlock(blockNum, pendingBlockContext)

	return nil
}

func (cache *RealtimeCache) tryCloseBlock(blockNum uint64, pendingBlockContext *PendingBlockContext) {
	if pendingBlockContext.txCount < 0 {
		// Header not received yet. Skip close
		return
	}

	if pendingBlockContext.pendingTxs.Size() > 0 || pendingBlockContext.txCount != int64(pendingBlockContext.nextTxIndex) {
		// Cannot close block yet, missing txs
		return
	}

	// Close block
	delete(cache.pendingBlocks, blockNum)
	if cache.highestConfirmHeight.Load() < blockNum {
		cache.highestConfirmHeight.Store(blockNum)
	}
}

func (cache *RealtimeCache) tryCreateNewPendingBlockContext(blockNum uint64) (*PendingBlockContext, error) {
	if len(cache.pendingBlocks) > PendingBlocksCacheSizeThreshold {
		return nil, fmt.Errorf("too many pending blocks, failed to process block msg and tx msgs. Pending blocks: %d", len(cache.pendingBlocks))
	}

	if pendingBlockContext, ok := cache.pendingBlocks[blockNum]; ok {
		return pendingBlockContext, nil
	}

	// Create new pending block context
	newPendingBlockContext := &PendingBlockContext{
		blockNum:    blockNum,
		nextTxIndex: 0,
		pendingTxs:  libcommon.NewOrderedList(DefaultTxMsgSliceSize, CompareTransactionMessages),
		txCount:     -1,
	}
	cache.pendingBlocks[blockNum] = newPendingBlockContext

	return newPendingBlockContext, nil
}
