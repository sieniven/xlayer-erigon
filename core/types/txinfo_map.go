package types

import (
	"sync"

	"github.com/ledgerwatch/erigon-lib/common"
)

type TxInfo struct {
	tx      Transaction
	receipt *Receipt
}

type TxInfoMap struct {
	txInfos map[common.Hash]TxInfo
	mu      sync.RWMutex
}

func NewTxInfoMap() *TxInfoMap {
	return &TxInfoMap{
		txInfos: make(map[common.Hash]TxInfo),
	}
}

// Get 根据交易哈希获取收据
func (rm *TxInfoMap) Get(txHash common.Hash) (Transaction, *Receipt, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	txInfo, exists := rm.txInfos[txHash]
	return txInfo.tx, txInfo.receipt, exists
}

// Put 插入交易哈希到收据的映射
func (rm *TxInfoMap) Put(txHash common.Hash, tx Transaction, receipt *Receipt) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.txInfos[txHash] = TxInfo{
		tx:      tx,
		receipt: receipt,
	}
}

// Delete 删除指定交易哈希的收据
func (rm *TxInfoMap) Delete(txHash common.Hash) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.txInfos, txHash)
}
