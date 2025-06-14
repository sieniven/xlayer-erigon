package realtime

import (
	"sync"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/realtime/kafka/types"
)

// -------------- Kafka Cache --------------
type KafkaCache struct {
	BlockMsgCache *BlockMessageCache
	TxMsgCache    *TransactionMessageCache
}

func NewKafkaCache() *KafkaCache {
	return &KafkaCache{
		BlockMsgCache: NewBlockMessageCache(),
		TxMsgCache:    NewTransactionMessageCache(),
	}
}

func (cache *KafkaCache) Clear() {
	cache.BlockMsgCache.Clear()
	cache.TxMsgCache.Clear()
}

func (cache *KafkaCache) Flush(blockNumber uint64) {
	if blockNumber == 0 {
		return
	}

	cache.BlockMsgCache.Flush(blockNumber)
	cache.TxMsgCache.Flush(blockNumber)
}

func (cache *KafkaCache) GetLowestBlockHeight() uint64 {
	return cache.BlockMsgCache.GetLowestBlockHeight()
}

func (cache *KafkaCache) GetHighestBlockHeight() uint64 {
	return cache.BlockMsgCache.GetHighestBlockHeight()
}

// -------------- Block Message Cache --------------
type BlockMessageCache struct {
	mu    sync.RWMutex
	cache map[uint64]*kafkaTypes.BlockMessage
}

func NewBlockMessageCache() *BlockMessageCache {
	return &BlockMessageCache{
		cache: make(map[uint64]*kafkaTypes.BlockMessage),
	}
}

func (cache *BlockMessageCache) Add(blockMsg *kafkaTypes.BlockMessage) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.cache[blockMsg.Header.Number.Uint64()] = blockMsg
}

func (cache *BlockMessageCache) Clear() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for k := range cache.cache {
		delete(cache.cache, k)
	}
}

func (cache *BlockMessageCache) Size() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return len(cache.cache)
}

// GetAndFlush pops the block message for the given block number
func (cache *BlockMessageCache) Pop(blockNumber uint64) (*kafkaTypes.BlockMessage, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	blockMsg, ok := cache.cache[blockNumber]
	delete(cache.cache, blockNumber)
	return blockMsg, ok
}

func (cache *BlockMessageCache) Flush(blockNumber uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for k := range cache.cache {
		if k <= blockNumber {
			delete(cache.cache, k)
		}
	}
}

func (cache *BlockMessageCache) GetLowestBlockHeight() uint64 {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	lowestBlockHeight := uint64(0)
	for k := range cache.cache {
		if lowestBlockHeight == 0 || k < lowestBlockHeight {
			lowestBlockHeight = k
		}
	}
	return lowestBlockHeight
}

func (cache *BlockMessageCache) GetHighestBlockHeight() uint64 {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	highestBlockHeight := uint64(0)
	for k := range cache.cache {
		if highestBlockHeight == 0 || k > highestBlockHeight {
			highestBlockHeight = k
		}
	}
	return highestBlockHeight
}

// -------------- Tx Message Cache --------------
type TransactionMessageCache struct {
	mu    sync.RWMutex
	cache map[uint64]*libcommon.OrderedList[*kafkaTypes.TransactionMessage]
}

func NewTransactionMessageCache() *TransactionMessageCache {
	return &TransactionMessageCache{
		cache: make(map[uint64]*libcommon.OrderedList[*kafkaTypes.TransactionMessage]),
	}
}

func (cache *TransactionMessageCache) Add(txMsg *kafkaTypes.TransactionMessage) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if _, ok := cache.cache[txMsg.BlockNumber]; !ok {
		cache.cache[txMsg.BlockNumber] = libcommon.NewOrderedList(DefaultTxMsgSliceSize, CompareTransactionMessages)
	}
	cache.cache[txMsg.BlockNumber].Add(txMsg)
	cache.cache[txMsg.BlockNumber].Sort()
}

func (cache *TransactionMessageCache) Clear() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for k := range cache.cache {
		delete(cache.cache, k)
	}
}

func (cache *TransactionMessageCache) Pop(blockNumber uint64) []*kafkaTypes.TransactionMessage {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	txMsg, ok := cache.cache[blockNumber]
	if !ok {
		return nil
	}

	// Retrieve all tx messages and clear the cache
	txs := make([]*kafkaTypes.TransactionMessage, txMsg.Size())
	copy(txs, txMsg.Items())
	txMsg.Clear()

	return txs
}

func (cache *TransactionMessageCache) Flush(blockNumber uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for k := range cache.cache {
		if k <= blockNumber {
			delete(cache.cache, k)
		}
	}
}

func NewOrderedListOfTransactionMessage(size int) *libcommon.OrderedList[*kafkaTypes.TransactionMessage] {
	return libcommon.NewOrderedList(size, CompareTransactionMessages)
}

func CompareTransactionMessages(a, b *kafkaTypes.TransactionMessage) int {
	if a.BlockNumber == b.BlockNumber {
		if a.Receipt != nil && b.Receipt != nil {
			return int(a.Receipt.TransactionIndex) - int(b.Receipt.TransactionIndex)
		}
		return 0
	}
	return int(a.BlockNumber) - int(b.BlockNumber)
}
