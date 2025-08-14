package vm

import (
	"math/big"
	"strings"
	"testing"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/ledgerwatch/erigon/core/types/accounts"
)

// MockStateReader implements state.StateReader for testing
type MockStateReader struct {
	accounts map[libcommon.Address]*accounts.Account
}

func NewMockStateReader() *MockStateReader {
	return &MockStateReader{
		accounts: make(map[libcommon.Address]*accounts.Account),
	}
}

func (m *MockStateReader) ReadAccountData(address libcommon.Address) (*accounts.Account, error) {
	if acc, exists := m.accounts[address]; exists {
		return acc, nil
	}
	return nil, nil
}

func (m *MockStateReader) ReadAccountStorage(address libcommon.Address, incarnation uint64, key *libcommon.Hash) ([]byte, error) {
	return nil, nil
}

func (m *MockStateReader) ReadAccountCode(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) ([]byte, error) {
	return nil, nil
}

func (m *MockStateReader) ReadAccountCodeSize(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) (int, error) {
	return 0, nil
}

func (m *MockStateReader) ReadAccountIncarnation(address libcommon.Address) (uint64, error) {
	return 0, nil
}

func (m *MockStateReader) SetAccount(address libcommon.Address, balance *uint256.Int, nonce uint64) {
	m.accounts[address] = &accounts.Account{
		Balance: *balance,
		Nonce:   nonce,
	}
}

func TestValidateMainnetBridgeAmount(t *testing.T) {
	tests := []struct {
		name            string
		targetAddress   libcommon.Address
		db              kv.RoDB
		maxBridgeAmount *big.Int
		expectedError   bool
	}{
		{
			name:            "Nil database",
			targetAddress:   libcommon.HexToAddress("0x1234567890123456789012345678901234567890"),
			db:              nil,
			maxBridgeAmount: big.NewInt(1000),
			expectedError:   false, // 现在只打印日志，不返回错误
		},
		{
			name:          "Valid database with sufficient amount",
			targetAddress: libcommon.HexToAddress("0x1234567890123456789012345678901234567890"),
			db:            createTestDB(),
			maxBridgeAmount: func() *big.Int {
				amount, _ := new(big.Int).SetString("300000000000000000000000000000000000000", 10) // smaller than per_mint_amount
				return amount
			}(),
			expectedError: false,
		},
		{
			name:            "Small bridge amount (should pass)",
			targetAddress:   libcommon.HexToAddress("0x1234567890123456789012345678901234567890"),
			db:              createTestDB(),
			maxBridgeAmount: big.NewInt(1000),
			expectedError:   false,
		},
		{
			name:          "Excessive bridge amount",
			targetAddress: libcommon.HexToAddress("0x1234567890123456789012345678901234567890"),
			db:            createTestDB(),
			maxBridgeAmount: func() *big.Int {
				amount, _ := new(big.Int).SetString("400282366920938463463374607431768211455", 10)
				return amount
			}(),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMainnetBridgeAmount(tt.targetAddress, tt.db, tt.maxBridgeAmount)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// createTestDB creates a test database for testing
func createTestDB() kv.RoDB {
	db := memdb.New("")
	return db
}

func TestInitEnvConfigWithBridgeAmount(t *testing.T) {
	tests := []struct {
		name             string
		rollupMgr        libcommon.Address
		maxBridgeAmount  *big.Int
		expectedError    bool
		expectedErrorMsg string
	}{
		{
			name:      "Valid mainnet with sufficient bridge amount",
			rollupMgr: libcommon.HexToAddress("0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2"), // mainnet rollup manager
			maxBridgeAmount: func() *big.Int {
				amount, _ := new(big.Int).SetString("300000000000000000000000000000000000000", 10) // smaller than per_mint_amount
				return amount
			}(), // very large amount
			expectedError: false,
		},
		{
			name:            "Valid testnet2 with nil bridge amount",
			rollupMgr:       libcommon.HexToAddress("0x32d33d5137a7cffb54c5bf8371172bcec5f310ff"), // testnet2 rollup manager
			maxBridgeAmount: nil,
			expectedError:   false,
		},
		{
			name:            "Small bridge amount (should pass)",
			rollupMgr:       libcommon.HexToAddress("0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2"), // mainnet rollup manager
			maxBridgeAmount: big.NewInt(1000),                                                     // small amount, less than remaining capacity
			expectedError:   false,
		},
		{
			name:      "Excessive bridge amount",
			rollupMgr: libcommon.HexToAddress("0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2"), // mainnet rollup manager
			maxBridgeAmount: func() *big.Int {
				amount, _ := new(big.Int).SetString("400282366920938463463374607431768211455", 10)
				return amount
			}(), // larger than per_mint_amount
			expectedError:    true,
			expectedErrorMsg: "exceeds remaining capacity",
		},
		{
			name:             "Nil database",
			rollupMgr:        libcommon.HexToAddress("0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2"), // mainnet rollup manager
			maxBridgeAmount:  big.NewInt(1000),
			expectedError:    false, // 现在只打印日志，不返回错误
			expectedErrorMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var db kv.RoDB
			if tt.expectedErrorMsg != "database is nil" {
				db = createTestDB()
			}

			err := InitEnvConfig(tt.rollupMgr, db, tt.maxBridgeAmount)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectedError && tt.expectedErrorMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.expectedErrorMsg) {
					t.Errorf("Expected error message to contain '%s', but got: %v", tt.expectedErrorMsg, err)
				}
			}
		})
	}
}
