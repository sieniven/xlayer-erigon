package types

import (
	"math/big"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeaderMap(t *testing.T) {
	hm := NewHeaderMap()

	blockNum := uint64(1)
	header := &Header{
		Number: big.NewInt(1),
		Time:   1000,
	}

	t.Run("Put and Get", func(t *testing.T) {
		hm.Put(blockNum, header)
		got, exists := hm.Get(blockNum)
		assert.True(t, exists)
		assert.Equal(t, header, got)
	})

	t.Run("Get non-existent", func(t *testing.T) {
		nonExistentNum := uint64(999)
		_, exists := hm.Get(nonExistentNum)
		assert.False(t, exists)
	})

	t.Run("Delete", func(t *testing.T) {
		hm.Delete(blockNum)
		_, exists := hm.Get(blockNum)
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
				header := &Header{
					Number: big.NewInt(int64(i)),
					Time:   uint64(i * 1000),
				}

				hm.Put(blockNum, header)

				got, exists := hm.Get(blockNum)
				assert.True(t, exists)
				assert.NotNil(t, got)
				assert.Equal(t, big.NewInt(int64(i)), got.Number)
				assert.Equal(t, uint64(i*1000), got.Time)

				hm.Delete(blockNum)

				_, exists = hm.Get(blockNum)
				assert.False(t, exists)
			}(i)
		}

		wg.Wait()

		for i := 0; i < goroutines; i++ {
			blockNum := uint64(i)
			_, exists := hm.Get(blockNum)
			assert.False(t, exists)
		}
	})
}
