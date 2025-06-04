package types

import (
	"sync"
	"testing"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/stretchr/testify/assert"
)

func TestReceiptMap(t *testing.T) {
	rm := NewReceiptMap()

	txHash := common.HexToHash("0x123")
	receipt := &Receipt{
		Status: 1,
	}

	t.Run("Put and Get", func(t *testing.T) {
		rm.Put(txHash, receipt)
		got, exists := rm.Get(txHash)
		assert.True(t, exists)
		assert.Equal(t, receipt, got)
	})

	t.Run("Get non-existent", func(t *testing.T) {
		nonExistentHash := common.HexToHash("0x456")
		_, exists := rm.Get(nonExistentHash)
		assert.False(t, exists)
	})

	t.Run("Delete", func(t *testing.T) {
		rm.Delete(txHash)
		_, exists := rm.Get(txHash)
		assert.False(t, exists)
	})

	t.Run("Concurrent operations", func(t *testing.T) {
		const goroutines = 10
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				hash := common.HexToHash(string(rune(i + 100)))
				receipt := &Receipt{Status: uint64(i)}

				rm.Put(hash, receipt)

				got, exists := rm.Get(hash)
				assert.True(t, exists)
				assert.NotNil(t, got)
				assert.Equal(t, uint64(i), got.Status)

				rm.Delete(hash)

				_, exists = rm.Get(hash)
				assert.False(t, exists)
			}(i)
		}

		wg.Wait()

		for i := 0; i < goroutines; i++ {
			hash := common.HexToHash(string(rune(i + 100)))
			_, exists := rm.Get(hash)
			assert.False(t, exists)
		}
	})
}
