package types

import (
	"sync"

	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type BlockInfo struct {
	Header    *ethTypes.Header
	TxCount   int64
	Completed bool
}

type BlockInfoMap struct {
	blockInfos map[uint64]*BlockInfo
	mu         sync.RWMutex
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
	bm.blockInfos[blockNum] = &BlockInfo{
		Header:  header,
		TxCount: -1,
	}
}

func (bm *BlockInfoMap) PutTxCount(blockNum uint64, txCount int64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	blockInfo, exists := bm.blockInfos[blockNum]
	if exists {
		bm.blockInfos[blockNum] = &BlockInfo{
			Header:  blockInfo.Header,
			TxCount: txCount,
		}
	} else {
		bm.blockInfos[blockNum] = &BlockInfo{
			TxCount: txCount,
		}
	}
}

func (bm *BlockInfoMap) Delete(blockNum uint64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	delete(bm.blockInfos, blockNum)
}

func (bm *BlockInfoMap) GetLastIncomplete(blockNumber, lastFinishHeight uint64) uint64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	if blockNumber == 0 {
		return lastFinishHeight + 1
	}

	temp := blockNumber
	for temp > lastFinishHeight {
		if bm.blockInfos[temp] == nil || bm.blockInfos[temp].Completed {
			return min(temp+1, blockNumber)
		}
		temp--
	}
	return lastFinishHeight + 1
}

func (bm *BlockInfoMap) MarkCompleted(blockNumber uint64) {
	bm.blockInfos[blockNumber].Completed = true
}

func (bm *BlockInfoMap) IsCompleted(blockNumber uint64) bool {
	blockInfo, exists := bm.blockInfos[blockNumber]
	if !exists {
		return true
	}
	return blockInfo.Completed
}
