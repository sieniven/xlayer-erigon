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
	txInfos map[common.Hash]TxInfo
	mu      sync.RWMutex
}

func NewTxInfoMap() *TxInfoMap {
	return &TxInfoMap{
		txInfos: make(map[common.Hash]TxInfo),
	}
}

func (rm *TxInfoMap) Get(txHash common.Hash) (ethTypes.Transaction, *ethTypes.Receipt, []*InnerTx, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	txInfo, exists := rm.txInfos[txHash]
	return txInfo.Tx, txInfo.Receipt, txInfo.InnerTxs, exists
}

func (rm *TxInfoMap) Put(txHash common.Hash, tx ethTypes.Transaction, receipt *ethTypes.Receipt, innerTxs []*InnerTx) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.txInfos[txHash] = TxInfo{
		Tx:       tx,
		Receipt:  receipt,
		InnerTxs: innerTxs,
	}
}

func (rm *TxInfoMap) Delete(txHash common.Hash) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.txInfos, txHash)
}
