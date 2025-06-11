package types

import (
	"sync/atomic"

	"github.com/ledgerwatch/erigon-lib/common"
	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type StatelessCache struct {
	highestHeight atomic.Uint64
	headerMap     *HeaderMap
	txInfoMap     *TxInfoMap
}

func NewStatelessCache() *StatelessCache {
	return &StatelessCache{
		headerMap: NewHeaderMap(),
		txInfoMap: NewTxInfoMap(),
	}
}

// -------------- Read operations --------------
func (cache *StatelessCache) GetHeight() uint64 {
	return cache.highestHeight.Load()
}

func (cache *StatelessCache) GetHeader(blockNum uint64) (*ethTypes.Header, bool) {
	return cache.headerMap.Get(blockNum)
}

func (cache *StatelessCache) GetTxInfo(txHash common.Hash) (ethTypes.Transaction, *ethTypes.Receipt, []*InnerTx, bool) {
	return cache.txInfoMap.GetTx(txHash)
}

func (cache *StatelessCache) GetBlockTxs(blockNum uint64) []common.Hash {
	return cache.txInfoMap.GetBlockTxs(blockNum)
}

func (cache *StatelessCache) PutHeader(blockNum uint64, header *ethTypes.Header) {
	if blockNum > cache.highestHeight.Load() {
		cache.highestHeight.Store(blockNum)
	}
	cache.headerMap.Put(blockNum, header)
}

// -------------- Write operations --------------
func (cache *StatelessCache) PutTxInfo(blockNum uint64, txHash common.Hash, tx ethTypes.Transaction, receipt *ethTypes.Receipt, innerTxs []*InnerTx) {
	if blockNum > cache.highestHeight.Load() {
		cache.highestHeight.Store(blockNum)
	}
	cache.txInfoMap.Put(blockNum, txHash, tx, receipt, innerTxs)
}

func (cache *StatelessCache) DeleteBlock(blockNum uint64, block *ethTypes.Block) {
	cache.headerMap.Delete(blockNum)
	cache.txInfoMap.DeleteBlockTxs(blockNum)

	for _, tx := range block.Transactions() {
		cache.txInfoMap.DeleteTxInfo(tx.Hash())
	}
}
