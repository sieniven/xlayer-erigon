package types

import (
	"math/big"
	"testing"

	ethTypes "github.com/ledgerwatch/erigon/core/types"
	"github.com/stretchr/testify/assert"
)

func TestBlockInfoMap(t *testing.T) {
	bm := NewBlockInfoMap(DefaultBlockCacheSize)

	blockNum := uint64(2)
	prevBlockNum := blockNum - 1
	header := &ethTypes.Header{
		Number: big.NewInt(int64(blockNum)),
		Time:   1000,
	}
	prevTxCount := int64(10)

	t.Run("PutHeader and Get", func(t *testing.T) {
		bm.PutHeader(blockNum, header, prevTxCount)
		gotHeader, gotTxCount, exists := bm.Get(blockNum)
		assert.True(t, exists)
		assert.Equal(t, header, gotHeader)
		// Init txCount is -1
		assert.Equal(t, int64(-1), gotTxCount)

		// Check previous block txCount
		gotHeader, gotTxCount, exists = bm.Get(prevBlockNum)
		assert.True(t, exists)
		assert.Nil(t, gotHeader)
		assert.Equal(t, gotTxCount, prevTxCount)
	})

	t.Run("Get non-existent", func(t *testing.T) {
		nonExistentNum := uint64(888)
		_, _, exists := bm.Get(nonExistentNum)
		assert.False(t, exists)
	})

	t.Run("Delete", func(t *testing.T) {
		bm.Delete(blockNum)
		_, _, exists := bm.Get(blockNum)
		assert.False(t, exists)
	})

	t.Run("Incremental operations", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			blockNum := uint64(i)
			prevBlockNum := blockNum - 1
			header := &ethTypes.Header{
				Number: big.NewInt(int64(i)),
				Time:   uint64(i * 1000),
			}
			prevTxCount := int64(i * 5)

			// Test PutHeader
			bm.PutHeader(blockNum, header, prevTxCount)
			gotHeader, gotTxCount, exists := bm.Get(blockNum)
			assert.True(t, exists)
			assert.NotNil(t, gotHeader)
			assert.Equal(t, big.NewInt(int64(i)), gotHeader.Number)
			assert.Equal(t, uint64(i*1000), gotHeader.Time)
			// Init txCount is -1
			assert.Equal(t, int64(-1), gotTxCount)

			// Check previous block txCount
			_, gotTxCount, exists = bm.Get(prevBlockNum)
			assert.True(t, exists)
			assert.Equal(t, gotTxCount, prevTxCount)

			// Test delete
			bm.Delete(blockNum)
			_, _, exists = bm.Get(blockNum)
			assert.False(t, exists)
		}
	})
}
