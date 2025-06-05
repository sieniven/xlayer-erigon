package types

import (
	"sync"

	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type HeaderMap struct {
	headers map[uint64]*ethTypes.Header
	mu      sync.RWMutex
}

func NewHeaderMap() *HeaderMap {
	return &HeaderMap{
		headers: make(map[uint64]*ethTypes.Header),
	}
}

func (hm *HeaderMap) Get(blockNum uint64) (*ethTypes.Header, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	header, exists := hm.headers[blockNum]
	return header, exists
}

func (hm *HeaderMap) Put(blockNum uint64, header *ethTypes.Header) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.headers[blockNum] = header
}

func (hm *HeaderMap) Delete(blockNum uint64) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.headers, blockNum)
}
