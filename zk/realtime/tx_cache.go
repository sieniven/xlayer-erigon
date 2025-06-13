package realtime

import (
	"sync"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	kafkaTypes "github.com/ledgerwatch/erigon/zk/realtime/kafka/types"
)

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

type TransactionMessageCache struct {
	cache map[uint64]*libcommon.OrderedList[*kafkaTypes.TransactionMessage]
	mu    sync.RWMutex
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
