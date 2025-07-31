package vm

import (
	"errors"
	"fmt"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/params"
)

var CONFIG_CONTRACT_MANAGER_ADDRESS = libcommon.HexToAddress("0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab")

// Operation codes for different token operations
const (
	TEST_OP = 0x01 // Test precompile availability (no authentication required)
	MINT_OP = 0x02 // Mint tokens
	BURN_OP = 0x03 // Burn tokens
)

// mintBurnPrecompile handles atomic mint and burn operations
// This precompile is framework-agnostic and only performs basic token operations
type mintBurnPrecompile struct {
	evm     *EVM
	enabled bool
	caller  libcommon.Address // Current caller address
}

// RequiredGas returns the required gas for mint/burn operations
func (c *mintBurnPrecompile) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return 0
	}

	operation := input[0]
	switch operation {
	case MINT_OP, BURN_OP:
		return params.SstoreSetGas
	default:
		return 0
	}
}

// Run executes the mint/burn precompile
// input format: [operation:1][data:64] (65 bytes total)
// - operation: 1 byte (0x01=mint, 0x02=burn)
// - data: 64 bytes (32-byte address + 32-byte amount)
func (c *mintBurnPrecompile) Run(input []byte) ([]byte, error) {
	if !c.enabled {
		return []byte{}, ErrUnsupportedPrecompile
	}

	// Validate minimum input length: at least 1 byte for operation
	if len(input) == 0 {
		return []byte{}, errors.New("empty input")
	}

	// Extract operation code (first byte)
	operation := input[0]

	// Check for valid operations first
	switch operation {
	case TEST_OP:
		// Test operation - no authentication required
		// Simply return success to indicate precompile is available
		return []byte("OK"), nil
	case MINT_OP, BURN_OP:
		// Only accept calls from the contract manager for valid operations
		if c.caller != CONFIG_CONTRACT_MANAGER_ADDRESS {
			return []byte{}, errors.New("unauthorized: only contract manager can call")
		}

		// Validate that there is data after operation code
		if len(input) <= 1 {
			return []byte{}, errors.New("missing operation data")
		}

		return c.handleTokenOperation(operation, input[1:])
	default:
		return nil, errors.New("invalid operation")
	}
}

// handleTokenOperation handles mint/burn operations
// data format: [address:32][amount:32] (64 bytes total)
// - address: 32-byte encoded (first 12 bytes padding, last 20 bytes actual address)
// - amount: 32-byte big-endian encoded uint256
func (c *mintBurnPrecompile) handleTokenOperation(operation byte, data []byte) ([]byte, error) {
	// Validate data length for mint/burn operations: exactly 64 bytes required
	if len(data) != 64 {
		return nil, fmt.Errorf("invalid data length for mint/burn: expected 64 bytes, got %d bytes", len(data))
	}

	// Extract address from bytes [12:32] (skip 12-byte padding)
	targetAddress := libcommon.BytesToAddress(data[12:32])
	// Extract amount from bytes [32:64]
	amount := new(uint256.Int).SetBytes(data[32:64])

	switch operation {
	case MINT_OP:
		return c.mintTokens(targetAddress, amount)
	case BURN_OP:
		return c.burnTokens(targetAddress, amount)
	}

	return nil, errors.New("invalid operation")
}

// mintTokens adds tokens to target address
func (c *mintBurnPrecompile) mintTokens(to libcommon.Address, amount *uint256.Int) ([]byte, error) {
	if amount == nil || amount.IsZero() {
		return nil, errors.New("invalid amount")
	}
	// Add balance directly to the address
	c.evm.IntraBlockState().AddBalance(to, amount)
	return []byte{}, nil
}

// burnTokens removes tokens from target address
func (c *mintBurnPrecompile) burnTokens(from libcommon.Address, amount *uint256.Int) ([]byte, error) {
	if amount == nil || amount.IsZero() {
		return nil, errors.New("invalid amount")
	}

	// Check if balance is sufficient
	currentBalance := c.evm.IntraBlockState().GetBalance(from)
	if currentBalance.Cmp(amount) < 0 {
		return nil, fmt.Errorf("insufficient balance")
	}

	// Subtract balance from the address
	c.evm.IntraBlockState().SubBalance(from, amount)
	return []byte{}, nil
}

// Required interface methods for precompile integration
func (c *mintBurnPrecompile) SetCounterCollector(cc *CounterCollector) {}
func (c *mintBurnPrecompile) SetOutputLength(outLength int)            {}
func (c *mintBurnPrecompile) SetEVM(evm *EVM) {
	c.evm = evm
}

// SetCaller sets the caller address
func (c *mintBurnPrecompile) SetCaller(caller libcommon.Address) {
	c.caller = caller
}
