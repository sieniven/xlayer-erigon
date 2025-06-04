package types

import (
	"sync"
	"testing"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/stretchr/testify/assert"
)

func TestTxInfoMap(t *testing.T) {
	tm := NewTxInfoMap()

	txHash := common.HexToHash("0x123")
	value := uint256.NewInt(0)
	gasPrice := uint256.NewInt(0)
	tx := NewTransaction(0, common.Address{}, value, 0, gasPrice, nil)
	receipt := &Receipt{
		Status: 1,
	}

	t.Run("Put and Get", func(t *testing.T) {
		tm.Put(txHash, tx, receipt)
		gotTx, gotReceipt, exists := tm.Get(txHash)
		assert.True(t, exists)
		assert.Equal(t, tx, gotTx)
		assert.Equal(t, receipt, gotReceipt)
	})

	t.Run("Get non-existent", func(t *testing.T) {
		nonExistentHash := common.HexToHash("0x456")
		_, _, exists := tm.Get(nonExistentHash)
		assert.False(t, exists)
	})

	t.Run("Delete", func(t *testing.T) {
		tm.Delete(txHash)
		_, _, exists := tm.Get(txHash)
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
				value := uint256.NewInt(uint64(i))
				gasPrice := uint256.NewInt(uint64(i))
				tx := NewTransaction(uint64(i), common.Address{}, value, uint64(i), gasPrice, nil)
				receipt := &Receipt{Status: uint64(i)}

				tm.Put(hash, tx, receipt)

				gotTx, gotReceipt, exists := tm.Get(hash)
				assert.True(t, exists)
				assert.NotNil(t, gotTx)
				assert.NotNil(t, gotReceipt)
				assert.Equal(t, uint64(i), gotReceipt.Status)

				tm.Delete(hash)

				_, _, exists = tm.Get(hash)
				assert.False(t, exists)
			}(i)
		}

		wg.Wait()

		for i := 0; i < goroutines; i++ {
			hash := common.HexToHash(string(rune(i + 100)))
			_, _, exists := tm.Get(hash)
			assert.False(t, exists)
		}
	})
}
