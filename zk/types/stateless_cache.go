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
	highestHeight atomic.Uint64
	blockInfoMap  *BlockInfoMap
	txInfoMap     *TxInfoMap
}

func NewStatelessCache() *StatelessCache {
	return &StatelessCache{
		highestHeight: atomic.Uint64{},
		blockInfoMap:  NewBlockInfoMap(),
		txInfoMap:     NewTxInfoMap(),
	}
}

// -------------- Read operations --------------
func (cache *StatelessCache) GetHeight() uint64 {
	return cache.highestHeight.Load()
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

func (cache *StatelessCache) GetLastCompleted() uint64 {
	return cache.blockInfoMap.GetLastCompleted()
}

func (cache *StatelessCache) GetLastIncomplete() uint64 {
	return cache.blockInfoMap.GetLastIncomplete()
}

// -------------- Write operations --------------
func (cache *StatelessCache) PutHeader(blockNum uint64, header *ethTypes.Header) {
	if blockNum > cache.highestHeight.Load() {
		cache.highestHeight.Store(blockNum)
	}
	cache.blockInfoMap.PutHeader(blockNum, header)
}

func (cache *StatelessCache) PutTxInfo(blockNum uint64, txHash libcommon.Hash, tx ethTypes.Transaction, receipt *ethTypes.Receipt, innerTxs []*InnerTx) {
	if blockNum > cache.highestHeight.Load() {
		cache.highestHeight.Store(blockNum)
	}
	cache.txInfoMap.Put(blockNum, txHash, tx, receipt, innerTxs)
}

func (cache *StatelessCache) PutTxCount(blockNum uint64, txCount int64) {
	if blockNum > cache.highestHeight.Load() {
		cache.highestHeight.Store(blockNum)
	}
	cache.blockInfoMap.PutTxCount(blockNum, txCount)
}

func (cache *StatelessCache) DeleteBlock(blockNum uint64, block *ethTypes.Block) {
	cache.blockInfoMap.Delete(blockNum)
	cache.txInfoMap.DeleteBlockTxs(blockNum)

	for _, tx := range block.Transactions() {
		cache.txInfoMap.DeleteTxInfo(tx.Hash())
	}
}

func (cache *StatelessCache) MarkCompleted(blockNum uint64) {
	cache.blockInfoMap.MarkCompleted(blockNum)
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
