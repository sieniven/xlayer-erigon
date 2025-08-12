package vm

import (
	"errors"
	"fmt"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/params"
)

var (
	ConfigContractManagerAddress = "0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab" // TODO, will set default value for mainnet
	TargetAddress                = "0x000000000000000000000000000000000000dEaD" // TODO, will set default value for mainnet
)

var (
	CONFIG_CONTRACT_MANAGER_ADDRESS = libcommon.HexToAddress(ConfigContractManagerAddress)
	TARGET_ADDRESS                  = libcommon.HexToAddress(TargetAddress)
)

// Operation codes for different token operations
const (
	TEST_OP  = 0x01 // Test precompile availability (no authentication required)
	MINT_OP  = 0x02 // Mint tokens
	CLEAN_OP = 0x03 // Clean up tokens from target address
)

type tokenManagerPrecompile struct {
	evm     *EVM
	enabled bool
	caller  libcommon.Address
}

func (c *tokenManagerPrecompile) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return 0
	}

	operation := input[0]
	switch operation {
	case MINT_OP:
		return params.SstoreSetGas
	case CLEAN_OP:
		return params.SstoreResetGas
	default:
		return 0
	}
}

func (c *tokenManagerPrecompile) Run(input []byte) ([]byte, error) {
	if !c.enabled {
		return []byte{}, ErrUnsupportedPrecompile
	}

	if len(input) == 0 {
		return []byte{}, errors.New("empty input")
	}

	operation := input[0]

	switch operation {
	case TEST_OP:
		return []byte("OK"), nil

	case MINT_OP:
		if c.caller != CONFIG_CONTRACT_MANAGER_ADDRESS {
			return []byte{}, errors.New("unauthorized: only contract manager can call")
		}
		if len(input) <= 1 {
			return []byte{}, errors.New("missing mint data")
		}
		return c.handleMint(input[1:])

	case CLEAN_OP:
		if c.caller != CONFIG_CONTRACT_MANAGER_ADDRESS {
			return []byte{}, errors.New("unauthorized: only contract manager can call")
		}
		// Get current balance
		balance := c.evm.IntraBlockState().GetBalance(TARGET_ADDRESS)

		one := uint256.NewInt(1)
		if balance.Cmp(one) <= 0 {
			return []byte{}, nil
		}

		// Keep 1 wei to maintain address existence in state trie
		// Prevent potential issues with zero-balance account deletion in some EVM implementations
		amountToClean := new(uint256.Int).Sub(balance, one)
		c.evm.IntraBlockState().SubBalance(TARGET_ADDRESS, amountToClean)
		return []byte{}, nil

	default:
		return nil, errors.New("invalid operation")
	}
}

func (c *tokenManagerPrecompile) handleMint(data []byte) ([]byte, error) {
	if len(data) != 64 {
		return nil, fmt.Errorf("invalid data length for mint: expected 64 bytes, got %d bytes", len(data))
	}

	targetAddress := libcommon.BytesToAddress(data[12:32])
	amount := new(uint256.Int).SetBytes(data[32:64])

	if amount.IsZero() {
		return nil, errors.New("invalid amount")
	}

	c.evm.IntraBlockState().AddBalance(targetAddress, amount)
	return []byte{}, nil
}

func (c *tokenManagerPrecompile) SetCounterCollector(cc *CounterCollector) {}
func (c *tokenManagerPrecompile) SetOutputLength(outLength int)            {}
func (c *tokenManagerPrecompile) SetEVM(evm *EVM) {
	c.evm = evm
}
func (c *tokenManagerPrecompile) SetCaller(caller libcommon.Address) {
	c.caller = caller
}
