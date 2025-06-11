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
	blockTxs map[uint64][]common.Hash
	mu       sync.RWMutex
}

func NewTxInfoMap() *TxInfoMap {
	return &TxInfoMap{
		txInfos:  make(map[common.Hash]TxInfo),
		blockTxs: make(map[uint64][]common.Hash),
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
	rm.blockTxs[blockNumber] = append(rm.blockTxs[blockNumber], txHash)
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

func (rm *TxInfoMap) GetTx(txHash common.Hash) (ethTypes.Transaction, *ethTypes.Receipt, []*InnerTx, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	txInfo, exists := rm.txInfos[txHash]
	return txInfo.Tx, txInfo.Receipt, txInfo.InnerTxs, exists
}

func (rm *TxInfoMap) GetBlockTxs(blockNumber uint64) []common.Hash {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	hashes, exists := rm.blockTxs[blockNumber]
	if !exists {
		return nil
	}
	return hashes
}
