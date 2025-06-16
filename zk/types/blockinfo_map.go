package types

import (
	"sync"

	ethTypes "github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/zkevm/log"
)

type BlockInfo struct {
	Header  *ethTypes.Header
	TxCount int64
}

type BlockInfoMap struct {
	blockInfos    map[uint64]*BlockInfo
	lastCompleted uint64
	mu            sync.RWMutex
}

func NewBlockInfoMap() *BlockInfoMap {
	return &BlockInfoMap{
		blockInfos: make(map[uint64]*BlockInfo),
	}
}

func (bm *BlockInfoMap) Get(blockNum uint64) (*ethTypes.Header, int64, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	blockInfo, exists := bm.blockInfos[blockNum]
	if exists {
		return blockInfo.Header, blockInfo.TxCount, true
	}
	return nil, 0, exists
}

func (bm *BlockInfoMap) PutHeader(blockNum uint64, header *ethTypes.Header) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	blockInfo, exists := bm.blockInfos[blockNum]
	if exists {
		blockInfo.Header = header
	} else {
		bm.blockInfos[blockNum] = &BlockInfo{
			Header:  header,
			TxCount: -1,
		}
	}
}

func (bm *BlockInfoMap) PutTxCount(blockNum uint64, txCount int64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	blockInfo, exists := bm.blockInfos[blockNum]
	if exists {
		blockInfo.TxCount = txCount
	} else {
		bm.blockInfos[blockNum] = &BlockInfo{
			Header:  nil,
			TxCount: txCount,
		}
	}
}

func (bm *BlockInfoMap) Delete(blockNum uint64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	delete(bm.blockInfos, blockNum)
	bm.MarkCompleted(blockNum)
}

func (bm *BlockInfoMap) GetLastIncomplete() uint64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return bm.lastCompleted + 1
}

func (bm *BlockInfoMap) MarkCompleted(blockNumber uint64) {
	if blockNumber > bm.lastCompleted {
		bm.lastCompleted = blockNumber
		log.Info("LastCompleted updated: ", bm.lastCompleted)
	}
}

func (bm *BlockInfoMap) GetLastCompleted() uint64 {
	return bm.lastCompleted
}
