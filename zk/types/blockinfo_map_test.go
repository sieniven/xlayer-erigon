package types

import (
	"math/big"
	"sync"
	"testing"

	ethTypes "github.com/ledgerwatch/erigon/core/types"
	"github.com/stretchr/testify/assert"
)

func TestBlockInfoMap(t *testing.T) {
	bm := NewBlockInfoMap()

	blockNum := uint64(1)
	header := &ethTypes.Header{
		Number: big.NewInt(1),
		Time:   1000,
	}
	txCount := int64(10)

	t.Run("PutHeader and Get", func(t *testing.T) {
		bm.PutHeader(blockNum, header)
		gotHeader, gotTxCount, exists := bm.Get(blockNum)
		assert.True(t, exists)
		assert.Equal(t, header, gotHeader)
		assert.Equal(t, uint64(0), gotTxCount) // 初始TxCount应该为0
	})

	t.Run("PutTxCount and Get", func(t *testing.T) {
		bm.PutTxCount(blockNum, txCount)
		gotHeader, gotTxCount, exists := bm.Get(blockNum)
		assert.True(t, exists)
		assert.Equal(t, header, gotHeader)
		assert.Equal(t, txCount, gotTxCount)
	})

	t.Run("PutTxCount for non-existent block", func(t *testing.T) {
		nonExistentNum := uint64(999)
		bm.PutTxCount(nonExistentNum, txCount)
		gotHeader, gotTxCount, exists := bm.Get(nonExistentNum)
		assert.True(t, exists)
		assert.Nil(t, gotHeader)
		assert.Equal(t, txCount, gotTxCount)
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

	t.Run("Concurrent operations", func(t *testing.T) {
		const goroutines = 10
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				blockNum := uint64(i)
				header := &ethTypes.Header{
					Number: big.NewInt(int64(i)),
					Time:   uint64(i * 1000),
				}
				txCount := int64(i * 5)

				// 测试PutHeader
				bm.PutHeader(blockNum, header)
				gotHeader, gotTxCount, exists := bm.Get(blockNum)
				assert.True(t, exists)
				assert.NotNil(t, gotHeader)
				assert.Equal(t, big.NewInt(int64(i)), gotHeader.Number)
				assert.Equal(t, uint64(i*1000), gotHeader.Time)
				assert.Equal(t, uint64(0), gotTxCount)

				// 测试PutTxCount
				bm.PutTxCount(blockNum, txCount)
				gotHeader, gotTxCount, exists = bm.Get(blockNum)
				assert.True(t, exists)
				assert.NotNil(t, gotHeader)
				assert.Equal(t, txCount, gotTxCount)

				// 测试Delete
				bm.Delete(blockNum)
				_, _, exists = bm.Get(blockNum)
				assert.False(t, exists)
			}(i)
		}

		wg.Wait()

		// 验证所有数据都被删除
		for i := 0; i < goroutines; i++ {
			blockNum := uint64(i)
			_, _, exists := bm.Get(blockNum)
			assert.False(t, exists)
		}
	})
}
