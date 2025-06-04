package types

import (
	"sync"

	"github.com/ledgerwatch/erigon-lib/common"
)

type ReceiptMap struct {
	receipts map[common.Hash]*Receipt
	mu       sync.RWMutex
}

func NewReceiptMap() *ReceiptMap {
	return &ReceiptMap{
		receipts: make(map[common.Hash]*Receipt),
	}
}

// Get 根据交易哈希获取收据
func (rm *ReceiptMap) Get(txHash common.Hash) (*Receipt, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	receipt, exists := rm.receipts[txHash]
	return receipt, exists
}

// Put 插入交易哈希到收据的映射
func (rm *ReceiptMap) Put(txHash common.Hash, receipt *Receipt) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.receipts[txHash] = receipt
}

// Delete 删除指定交易哈希的收据
func (rm *ReceiptMap) Delete(txHash common.Hash) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.receipts, txHash)
}
