package vm

import (
	"errors"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/params"
)

// Token manager caller address (hardcoded)
var TOKEN_MANAGER_CALLER = libcommon.HexToAddress("0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a")

// Operation codes definition
const (
	// Basic operations
	TOKEN_MINT_OP = 0x01 // Token minting operation
	TOKEN_BURN_OP = 0x02 // Token burning operation

	QUERY_CALLER_OP = 0x20 // Query current hardcoded token manager caller
)

// Token Manager precompile contract
type tokenManager_zkevm struct {
	enabled bool
	evm     *EVM
	caller  ContractRef
}

func (c *tokenManager_zkevm) SetCounterCollector(cc *CounterCollector) {
}

func (c *tokenManager_zkevm) SetOutputLength(outLength int) {
}

func (c *tokenManager_zkevm) SetContext(evm *EVM, caller ContractRef) {
	c.evm = evm
	c.caller = caller
}

// RequiredGas calculates the required gas fee
func (c *tokenManager_zkevm) RequiredGas(input []byte) uint64 {
	if !c.enabled {
		return 0
	}

	if len(input) < 1 {
		return 0
	}

	operation := input[0]
	switch operation {
	case TOKEN_MINT_OP:
		return params.TokenMintGas
	case TOKEN_BURN_OP:
		return params.TokenBurnGas
	case QUERY_CALLER_OP:
		return params.TokenQueryGas
	default:
		return 0
	}
}

// Run executes the precompiled contract with given input bytes
// Input format: 20 bytes (to address) + 32 bytes (amount uint256) = 52 bytes total
func (c *tokenManager_zkevm) Run(input []byte) ([]byte, error) {
	if !c.enabled {
		return nil, ErrUnsupportedPrecompile
	}

	if len(input) < 1 {
		return nil, errors.New("invalid input length")
	}

	operation := input[0]
	switch operation {
	case TOKEN_MINT_OP, TOKEN_BURN_OP:
		return c.handleTokenOperation(operation, input[1:])
	case QUERY_CALLER_OP:
		return c.handleQueryCaller()
	default:
		return nil, errors.New("invalid operation")
	}
}

// isCaller checks if the caller is the hardcoded caller
func (c *tokenManager_zkevm) isCaller() bool {
	return c.caller.Address() == TOKEN_MANAGER_CALLER
}

// handleQueryCaller handles querying the hardcoded caller
func (c *tokenManager_zkevm) handleQueryCaller() ([]byte, error) {
	return TOKEN_MANAGER_CALLER.Bytes(), nil
}

// handleTokenOperation handles token operations
func (c *tokenManager_zkevm) handleTokenOperation(operation byte, data []byte) ([]byte, error) {
	if len(data) != 52 { // 20 bytes address + 32 bytes amount
		return nil, errors.New("invalid token operation data")
	}

	// Check if caller is the hardcoded caller
	if !c.isCaller() {
		return nil, errors.New("unauthorized: only hardcoded caller can perform token operations")
	}

	targetAddress := libcommon.BytesToAddress(data[:20])
	amount := new(uint256.Int).SetBytes(data[20:52])

	switch operation {
	case TOKEN_MINT_OP:
		return c.mintTokens(targetAddress, amount)
	case TOKEN_BURN_OP:
		return c.burnTokens(targetAddress, amount)
	}

	return nil, errors.New("invalid token operation")
}

// mintTokens mints tokens
func (c *tokenManager_zkevm) mintTokens(targetAddress libcommon.Address, amount *uint256.Int) ([]byte, error) {
	if amount.IsZero() {
		return nil, errors.New("mint amount cannot be zero")
	}

	// Prevent overflow - check target address current balance
	currentBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	maxUint256 := new(uint256.Int).SetAllOne()
	// Check if adding amount would cause overflow
	if currentBalance.Cmp(new(uint256.Int).Sub(maxUint256, amount)) > 0 {
		return nil, errors.New("mint amount would cause overflow")
	}

	// Execute minting
	c.evm.intraBlockState.AddBalance(targetAddress, amount)

	// Emit TokenMinted event
	c.emitTokenMinted(targetAddress, amount)

	return nil, nil
}

// burnTokens burns tokens
func (c *tokenManager_zkevm) burnTokens(targetAddress libcommon.Address, amount *uint256.Int) ([]byte, error) {
	if amount.IsZero() {
		return nil, errors.New("burn amount cannot be zero")
	}

	// Check if balance is sufficient
	currentBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	if currentBalance.Cmp(amount) < 0 {
		return nil, errors.New("insufficient balance for burn")
	}

	// Prevent burning entire balance to avoid EIP-161 account deletion
	// which could cause sequencer issues when account is removed from state
	if currentBalance.Cmp(amount) == 0 {
		return nil, errors.New("cannot burn entire balance - account would be deleted by EIP-161")
	}

	// Execute burning
	c.evm.intraBlockState.SubBalance(targetAddress, amount)

	// Emit TokenBurned event
	c.emitTokenBurned(targetAddress, amount)

	return nil, nil
}

// emitTokenMinted emits TokenMinted event
func (c *tokenManager_zkevm) emitTokenMinted(to libcommon.Address, amount *uint256.Int) {
	// Emit Minted event: Minted(indexed address, uint256)
	// Topic0: keccak256("Minted(address,uint256)")
	// Topic1: indexed address (target address)
	// Data: amount (uint256)
	mintEventSig := crypto.Keccak256Hash([]byte("Minted(address,uint256)"))
	toAddress := libcommon.BytesToHash(to.Bytes())
	topics := []libcommon.Hash{
		mintEventSig,
		toAddress,
	}

	data := common.LeftPadBytes(amount.Bytes(), 32)

	c.emitLog(topics, data)
}

// emitTokenBurned emits TokenBurned event
func (c *tokenManager_zkevm) emitTokenBurned(from libcommon.Address, amount *uint256.Int) {
	// Emit Burned event: Burned(indexed address, uint256)
	// Topic0: keccak256("Burned(address,uint256)")
	// Topic1: indexed address (from address)
	// Data: amount (uint256)
	burnEventSig := crypto.Keccak256Hash([]byte("Burned(address,uint256)"))
	fromAddress := libcommon.BytesToHash(from.Bytes())
	topics := []libcommon.Hash{
		burnEventSig,
		fromAddress,
	}

	data := common.LeftPadBytes(amount.Bytes(), 32)

	c.emitLog(topics, data)
}

// emitLog helper method for emitting logs
func (c *tokenManager_zkevm) emitLog(topics []libcommon.Hash, data []byte) {
	// Create log entry
	log := &types.Log{
		Address: c.getContractAddress(), // precompile contract address
		Topics:  topics,
		Data:    data,
	}

	// Add to EVM logs
	c.evm.intraBlockState.AddLog_zkEvm(log)
}

// getContractAddress gets the contract address
func (c *tokenManager_zkevm) getContractAddress() libcommon.Address {
	return libcommon.BytesToAddress([]byte{0x01, 0x01})
}
