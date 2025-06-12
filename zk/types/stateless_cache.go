package types

import (
	"context"
	"fmt"
	"sync/atomic"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type StatelessCache struct {
	blockInfoMap *BlockInfoMap
	txInfoMap    *TxInfoMap

	// highestCompletedHeight is the highest confirmed height from kafka
	highestCompletedHeight atomic.Uint64
	// highestExecutionHeight is the highest executed height on the RPC node
	highestExecutionHeight atomic.Uint64

	// nextTxIndex is the next tx index to be processed in the current pending block
	nextTxIndex uint64
}

func NewStatelessCache() *StatelessCache {
	return &StatelessCache{
		blockInfoMap:           NewBlockInfoMap(),
		txInfoMap:              NewTxInfoMap(),
		highestCompletedHeight: atomic.Uint64{},
		highestExecutionHeight: atomic.Uint64{},
		nextTxIndex:            0,
	}
}

func (cache *StatelessCache) Clear() {
	cache.nextTxIndex = 0
}

// -------------- Read operations --------------
func (cache *StatelessCache) GetCompletedHeight() uint64 {
	return cache.highestCompletedHeight.Load()
}

func (cache *StatelessCache) GetExecutionHeight() uint64 {
	return cache.highestExecutionHeight.Load()
}

func (cache *StatelessCache) GetHeader(blockNum uint64) (*ethTypes.Header, int64, bool) {
	return cache.blockInfoMap.Get(blockNum)
}

func (cache *StatelessCache) GetTxInfo(txHash libcommon.Hash) (ethTypes.Transaction, *ethTypes.Receipt, uint64, []*InnerTx, bool) {
	return cache.txInfoMap.GetTx(txHash)
}

func (cache *StatelessCache) GetBlockTxs(blockNum uint64) ([]libcommon.Hash, bool) {
	return cache.txInfoMap.GetBlockTxs(blockNum)
}

// -------------- Write operations --------------
func (cache *StatelessCache) PutHeader(blockNum uint64, header *ethTypes.Header, prevTxCount int64) {
	cache.blockInfoMap.PutHeader(blockNum, header, prevTxCount)
}

func (cache *StatelessCache) PutCompletedHeight(blockNum uint64) {
	if blockNum > cache.highestCompletedHeight.Load() {
		cache.highestCompletedHeight.Store(blockNum)
	}
}

func (cache *StatelessCache) PutExecutionHeight(blockNum uint64) {
	if blockNum > cache.highestExecutionHeight.Load() {
		cache.highestExecutionHeight.Store(blockNum)
	}
}

func (cache *StatelessCache) PutTxInfo(blockNum uint64, txHash libcommon.Hash, tx ethTypes.Transaction, receipt *ethTypes.Receipt, innerTxs []*InnerTx) {
	cache.txInfoMap.Put(blockNum, txHash, tx, receipt, innerTxs)
}

func (cache *StatelessCache) DeleteBlock(blockNum uint64, block *ethTypes.Block) {
	cache.blockInfoMap.Delete(blockNum)
	cache.txInfoMap.DeleteBlockTxs(blockNum)

	for _, tx := range block.Transactions() {
		cache.txInfoMap.DeleteTxInfo(tx.Hash())
	}
}

// -------------- For HeaderReader --------------
func (cache *StatelessCache) Header(ctx context.Context, tx kv.Getter, hash libcommon.Hash, blockNum uint64) (*ethTypes.Header, error) {
	header, _, ok := cache.GetHeader(blockNum)
	if !ok {
		return nil, fmt.Errorf("header not found for block number %d", blockNum)
	}
	return header, nil
}

func (cache *StatelessCache) HeaderByNumber(ctx context.Context, tx kv.Getter, blockNum uint64) (*ethTypes.Header, error) {
	header, _, ok := cache.GetHeader(blockNum)
	if !ok {
		return nil, fmt.Errorf("header not found for block number %d", blockNum)
	}
	return header, nil
}

func (cache *StatelessCache) HeaderByHash(ctx context.Context, tx kv.Getter, hash libcommon.Hash) (*ethTypes.Header, error) {
	// Unimplemented
	return nil, nil
}

func (cache *StatelessCache) ReadAncestor(db kv.Getter, hash libcommon.Hash, number, ancestor uint64, maxNonCanonical *uint64) (libcommon.Hash, uint64) {
	// Unimplemented
	return libcommon.Hash{}, 0
}

func (cache *StatelessCache) HeadersRange(ctx context.Context, walker func(header *ethTypes.Header) error) error {
	// Unimplemented
	return nil
}

func (cache *StatelessCache) Integrity(ctx context.Context) error {
	// Unimplemented
	return nil
}
