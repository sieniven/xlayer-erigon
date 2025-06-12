package types

import (
	"sync"

	"github.com/ledgerwatch/erigon-lib/common"
	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type TxInfo struct {
	BlockNumber uint64
	Tx          ethTypes.Transaction
	Receipt     *ethTypes.Receipt
	InnerTxs    []*InnerTx
	Changeset   *Changeset
}

type TxInfoMap struct {
	txInfos  map[common.Hash]TxInfo
	blockTxs map[uint64]map[common.Hash]struct{}
	mu       sync.RWMutex
}

func NewTxInfoMap() *TxInfoMap {
	return &TxInfoMap{
		txInfos:  make(map[common.Hash]TxInfo),
		blockTxs: make(map[uint64]map[common.Hash]struct{}),
	}
}

func (rm *TxInfoMap) Put(blockNumber uint64, txHash common.Hash, tx ethTypes.Transaction, receipt *ethTypes.Receipt, innerTxs []*InnerTx) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	txInfo := TxInfo{
		BlockNumber: blockNumber,
		Tx:          tx,
		Receipt:     receipt,
		InnerTxs:    innerTxs,
	}

	rm.txInfos[txHash] = txInfo
	if _, exists := rm.blockTxs[blockNumber]; !exists {
		rm.blockTxs[blockNumber] = make(map[common.Hash]struct{})
	}
	rm.blockTxs[blockNumber][txHash] = struct{}{}
}

func (rm *TxInfoMap) DeleteBlockTxs(blockNumber uint64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.blockTxs, blockNumber)
}

func (rm *TxInfoMap) DeleteTxInfo(txHash common.Hash) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.txInfos, txHash)
}

func (rm *TxInfoMap) GetTx(txHash common.Hash) (ethTypes.Transaction, *ethTypes.Receipt, uint64, []*InnerTx, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	txInfo, exists := rm.txInfos[txHash]
	return txInfo.Tx, txInfo.Receipt, txInfo.BlockNumber, txInfo.InnerTxs, exists
}

func (rm *TxInfoMap) GetBlockTxs(blockNumber uint64) ([]common.Hash, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	hashSet, exists := rm.blockTxs[blockNumber]
	if !exists {
		return nil, false
	}

	hashes := make([]common.Hash, 0, len(hashSet))
	for hash := range hashSet {
		hashes = append(hashes, hash)
	}

	return hashes, true
}
