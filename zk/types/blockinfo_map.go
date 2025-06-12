package types

import (
	"sync"

	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type BlockInfo struct {
	Header  *ethTypes.Header
	TxCount int64
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

func (bm *BlockInfoMap) PutHeader(blockNum uint64, header *ethTypes.Header, prevTxCount int64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.blockInfos[blockNum] = &BlockInfo{
		Header:  header,
		TxCount: -1,
	}

	// Update previous block header tx count
	blockInfo, exists := bm.blockInfos[blockNum-1]
	if exists {
		blockInfo.TxCount = prevTxCount
	} else {
		bm.blockInfos[blockNum] = &BlockInfo{
			TxCount: prevTxCount,
		}
	}
}

func (bm *BlockInfoMap) Delete(blockNum uint64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	delete(bm.blockInfos, blockNum)
}
