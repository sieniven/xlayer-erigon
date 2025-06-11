package types

import (
	"sync"

	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type BlockInfo struct {
	Header  *ethTypes.Header
	TxCount uint64
}

type BlockInfoMap struct {
	blockInfos map[uint64]BlockInfo
	mu         sync.RWMutex
}

func NewBlockInfoMap() *BlockInfoMap {
	return &BlockInfoMap{
		blockInfos: make(map[uint64]BlockInfo),
	}
}

func (bm *BlockInfoMap) Get(blockNum uint64) (*ethTypes.Header, uint64, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	blockInfo, exists := bm.blockInfos[blockNum]
	return blockInfo.Header, blockInfo.TxCount, exists
}

func (bm *BlockInfoMap) PutHeader(blockNum uint64, header *ethTypes.Header) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.blockInfos[blockNum] = BlockInfo{
		Header: header,
	}
}

func (bm *BlockInfoMap) PutTxCount(blockNum uint64, txCount uint64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	blockInfo, exists := bm.blockInfos[blockNum]
	if exists {
		bm.blockInfos[blockNum] = BlockInfo{
			Header:  blockInfo.Header,
			TxCount: txCount,
		}
	} else {
		bm.blockInfos[blockNum] = BlockInfo{
			TxCount: txCount,
		}
	}
}

func (bm *BlockInfoMap) Delete(blockNum uint64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	delete(bm.blockInfos, blockNum)
}
