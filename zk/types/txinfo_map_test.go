package types

import (
	"math/big"
	"sync"
	"testing"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	ethTypes "github.com/ledgerwatch/erigon/core/types"
	"github.com/stretchr/testify/assert"
)

func TestTxInfoMap(t *testing.T) {
	tm := NewTxInfoMap()

	txHash := common.HexToHash("0x123")
	value := uint256.NewInt(0)
	gasPrice := uint256.NewInt(0)
	tx := ethTypes.NewTransaction(0, common.Address{}, value, 0, gasPrice, nil)
	receipt := &ethTypes.Receipt{
		Status: 1,
	}
	innerTxs := []*InnerTx{
		{
			Dept:          *big.NewInt(1),
			InternalIndex: *big.NewInt(1),
			CallType:      "call",
			Name:          "call_1",
			TraceAddress:  "0",
			CodeAddress:   "0x123",
			From:          "0x456",
			To:            "0x789",
			Input:         "0x",
			Output:        "0x",
			IsError:       false,
			Gas:           21000,
			GasUsed:       21000,
			Value:         "0",
			ValueWei:      "0",
			CallValueWei:  "0",
			Error:         "",
		},
		{
			Dept:          *big.NewInt(1),
			InternalIndex: *big.NewInt(2),
			CallType:      "call",
			Name:          "call_2",
			TraceAddress:  "1",
			CodeAddress:   "0x123",
			From:          "0x456",
			To:            "0x789",
			Input:         "0x",
			Output:        "0x",
			IsError:       false,
			Gas:           21000,
			GasUsed:       21000,
			Value:         "0",
			ValueWei:      "0",
			CallValueWei:  "0",
			Error:         "",
		},
	}

	t.Run("Put and Get", func(t *testing.T) {
		tm.Put(txHash, tx, receipt, innerTxs)
		gotTx, gotReceipt, gotInnerTxs, exists := tm.Get(txHash)
		assert.True(t, exists)
		assert.Equal(t, tx, gotTx)
		assert.Equal(t, receipt, gotReceipt)
		assert.Equal(t, innerTxs, gotInnerTxs)
	})

	t.Run("Get non-existent", func(t *testing.T) {
		nonExistentHash := common.HexToHash("0x456")
		_, _, _, exists := tm.Get(nonExistentHash)
		assert.False(t, exists)
	})

	t.Run("Delete", func(t *testing.T) {
		tm.Delete(txHash)
		_, _, _, exists := tm.Get(txHash)
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
				tx := ethTypes.NewTransaction(uint64(i), common.Address{}, value, uint64(i), gasPrice, nil)
				receipt := &ethTypes.Receipt{Status: uint64(i)}
				innerTxs := []*InnerTx{
					{
						Dept:          *big.NewInt(int64(i)),
						InternalIndex: *big.NewInt(int64(i)),
						CallType:      "call",
						Name:          "call_1",
						TraceAddress:  "0",
						CodeAddress:   "0x123",
						From:          "0x456",
						To:            "0x789",
						Input:         "0x",
						Output:        "0x",
						IsError:       false,
						Gas:           uint64(i),
						GasUsed:       uint64(i),
						Value:         "0",
						ValueWei:      "0",
						CallValueWei:  "0",
						Error:         "",
					},
					{
						Dept:          *big.NewInt(int64(i)),
						InternalIndex: *big.NewInt(int64(i + 1)),
						CallType:      "call",
						Name:          "call_2",
						TraceAddress:  "1",
						CodeAddress:   "0x123",
						From:          "0x456",
						To:            "0x789",
						Input:         "0x",
						Output:        "0x",
						IsError:       false,
						Gas:           uint64(i),
						GasUsed:       uint64(i),
						Value:         "0",
						ValueWei:      "0",
						CallValueWei:  "0",
						Error:         "",
					},
				}

				tm.Put(hash, tx, receipt, innerTxs)

				gotTx, gotReceipt, gotInnerTxs, exists := tm.Get(hash)
				assert.True(t, exists)
				assert.NotNil(t, gotTx)
				assert.NotNil(t, gotReceipt)
				assert.Equal(t, uint64(i), gotReceipt.Status)
				assert.Equal(t, innerTxs, gotInnerTxs)

				tm.Delete(hash)

				_, _, _, exists = tm.Get(hash)
				assert.False(t, exists)
			}(i)
		}

		wg.Wait()

		for i := 0; i < goroutines; i++ {
			hash := common.HexToHash(string(rune(i + 100)))
			_, _, _, exists := tm.Get(hash)
			assert.False(t, exists)
		}
	})
}
