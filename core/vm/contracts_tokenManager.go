package vm

import (
	"errors"
	"fmt"
	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/params"
)

// Initial admin address (hardcoded)
var INITIAL_ADMIN = libcommon.HexToAddress("0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15")

// Token Manager precompile contract
type tokenManager_zkevm struct {
	enabled bool
	evm     *EVM
}

// Operation codes definition
const (
	// Basic operations
	TOKEN_MINT_OP = 0x01 // Token minting operation
	TOKEN_BURN_OP = 0x02 // Token burning operation

	// Admin management
	CHANGE_ADMIN_OP = 0x10 // Change admin address
	QUERY_ADMIN_OP  = 0x20 // Query current admin

	// Storage keys
	CURRENT_ADMIN_KEY = "TOKEN_MANAGER_CURRENT_ADMIN"
)

// Event signatures for logging
var (
	// TokenMinted(address indexed to, uint256 amount, address indexed admin)
	TokenMintedEventSig = libcommon.HexToHash("0xab8530f87dc9b59234c4623bf917212bb2536d647574c8e7e5da92c2ede0c9f8")

	// TokenBurned(address indexed from, uint256 amount, address indexed admin)
	TokenBurnedEventSig = libcommon.HexToHash("0xcc16f5dbb4873280815c1ee09dbd06736cffcc184412cf7a71a0fdb75d397ca5")

	// AdminChanged(address indexed oldAdmin, address indexed newAdmin)
	AdminChangedEventSig = libcommon.HexToHash("0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f")
)

// RequiredGas calculates the required gas fee
func (c *tokenManager_zkevm) RequiredGas(input []byte) uint64 {
	if !c.enabled {
		return 0
	}

	// If Token Manager is not yet active, consume 0 gas (like calling an empty account)
	if !c.isTokenManagerActive() {
		return 0
	}

	if len(input) < 1 {
		return 0
	}

	switch input[0] {
	case TOKEN_MINT_OP:
		return params.TokenMintGas
	case TOKEN_BURN_OP:
		return params.TokenBurnGas
	case CHANGE_ADMIN_OP:
		return params.TokenAdminGas
	case QUERY_ADMIN_OP:
		return params.TokenQueryGas
	default:
		return 0
	}
}

// isTokenManagerActive checks if Token Manager is active at current block height
func (c *tokenManager_zkevm) isTokenManagerActive() bool {
	if c.evm == nil {
		return false
	}

	//currentBlockNumber := c.evm.Context.BlockNumber
	//if c.evm.chainConfig != nil && c.evm.chainConfig.TokenManager != nil {
	//	config := c.evm.chainConfig.TokenManager
	//	if config.ActivationBlock != nil {
	//		return currentBlockNumber >= *config.ActivationBlock
	//	}
	//}

	return true
}

// SetCounterCollector sets the counter collector
func (c *tokenManager_zkevm) SetCounterCollector(cc *CounterCollector) {
	// Temporarily ignore CounterCollector
}

// SetOutputLength sets the output length
func (c *tokenManager_zkevm) SetOutputLength(outLength int) {
	// Temporarily ignore output length setting
}

// SetEVM sets the EVM reference
func (c *tokenManager_zkevm) SetEVM(evm *EVM) {
	c.evm = evm
}

// Run executes the precompile contract
func (c *tokenManager_zkevm) Run(input []byte) ([]byte, error) {
	fmt.Printf("Received input: %x\n", input)
	if !c.enabled {
		return []byte{}, ErrUnsupportedPrecompile
	}

	// If Token Manager is not yet active, behave like an empty account call
	// This ensures consensus compatibility with nodes that don't have Token Manager
	if !c.isTokenManagerActive() {
		return []byte{}, errors.New("token Manager not activated: missing configuration or activation block not reached") // Empty result, no error (same as calling empty account)
	}

	if len(input) < 1 {
		return nil, errors.New("invalid input length")
	}

	operation := input[0]
	switch operation {
	case TOKEN_MINT_OP, TOKEN_BURN_OP:
		return c.handleTokenOperation(operation, input[1:])
	case CHANGE_ADMIN_OP:
		return c.handleChangeAdmin(input[1:])
	case QUERY_ADMIN_OP:
		return c.handleQueryAdmin()
	default:
		return nil, errors.New("invalid operation")
	}
}

// getCurrentAdmin gets the current admin address
func (c *tokenManager_zkevm) getCurrentAdmin() libcommon.Address {
	adminKey := libcommon.BytesToHash([]byte(CURRENT_ADMIN_KEY))
	var adminBytes uint256.Int
	c.evm.intraBlockState.GetState(c.getContractAddress(), &adminKey, &adminBytes)

	fmt.Printf("查询管理员 - 合约地址: %s\n", c.getContractAddress().Hex())
	fmt.Printf("查询管理员 - Key: %s\n", adminKey.Hex())
	fmt.Printf("查询管理员 - 读取到: %s\n", adminBytes.Hex())
	fmt.Printf("查询管理员 - IsZero: %v\n", adminBytes.IsZero())

	// If no admin address is stored on-chain, it's the initial state, use hardcoded address
	if adminBytes.IsZero() {
		fmt.Printf("返回初始管理员: %s\n", INITIAL_ADMIN.Hex())
		return INITIAL_ADMIN
	}

	// Read current admin address from on-chain
	adminAddr := libcommon.Address{}
	adminBytes.WriteToSlice(adminAddr[:])
	fmt.Printf("返回存储的管理员: %s\n", adminAddr.Hex())
	return adminAddr
}

// setCurrentAdmin sets the current admin address
func (c *tokenManager_zkevm) setCurrentAdmin(newAdmin libcommon.Address) {
	adminKey := libcommon.BytesToHash([]byte(CURRENT_ADMIN_KEY))
	value := new(uint256.Int).SetBytes(newAdmin[:])

	fmt.Printf("设置管理员 - 合约地址: %s\n", c.getContractAddress().Hex())
	fmt.Printf("设置管理员 - Key: %s\n", adminKey.Hex())
	fmt.Printf("设置管理员 - Value: %s\n", value.Hex())

	c.evm.intraBlockState.SetState(c.getContractAddress(), &adminKey, *value)

	// 立即验证
	var readBack uint256.Int
	c.evm.intraBlockState.GetState(c.getContractAddress(), &adminKey, &readBack)
	fmt.Printf("立即读取验证: %s\n", readBack.Hex())
}

// isCurrentAdmin checks if the caller is the current admin
func (c *tokenManager_zkevm) isCurrentAdmin() bool {
	caller := c.evm.TxContext.Origin
	currentAdmin := c.getCurrentAdmin()
	return caller == currentAdmin
}

// handleTokenOperation handles token operations
func (c *tokenManager_zkevm) handleTokenOperation(operation byte, data []byte) ([]byte, error) {
	if len(data) < 64 { // 32 bytes address + 32 bytes amount
		return nil, errors.New("invalid token operation data")
	}

	// Check if caller is the current admin
	if !c.isCurrentAdmin() {
		return nil, errors.New("unauthorized: only current admin can perform token operations")
	}

	// Address is 32-byte encoded (first 12 bytes are padding, last 20 bytes are the actual address)
	targetAddress := libcommon.BytesToAddress(data[12:32])
	amount := new(uint256.Int).SetBytes(data[32:64])

	switch operation {
	case TOKEN_MINT_OP:
		return c.mintTokens(targetAddress, amount)
	case TOKEN_BURN_OP:
		return c.burnTokens(targetAddress, amount)
	}

	return nil, errors.New("invalid token operation")
}

// handleChangeAdmin handles admin change
func (c *tokenManager_zkevm) handleChangeAdmin(data []byte) ([]byte, error) {
	if len(data) < 32 { // 32 bytes new admin address
		return nil, errors.New("invalid change admin data")
	}

	// Check if caller is the current admin
	if !c.isCurrentAdmin() {
		return nil, errors.New("unauthorized: only current admin can change admin")
	}

	oldAdmin := c.getCurrentAdmin()
	// Address is 32-byte encoded (first 12 bytes are padding, last 20 bytes are the actual address)
	newAdmin := libcommon.BytesToAddress(data[12:32])

	// Prevent setting zero address as admin
	if newAdmin == (libcommon.Address{}) {
		return nil, errors.New("cannot set zero address as admin")
	}

	// Prevent setting the same admin
	if newAdmin == oldAdmin {
		return nil, errors.New("new admin cannot be the same as current admin")
	}

	// Change admin (old admin including hardcoded address becomes permanently invalid)
	c.setCurrentAdmin(newAdmin)

	// Emit AdminChanged event
	c.emitAdminChanged(oldAdmin, newAdmin)

	return []byte{1}, nil // Success
}

// handleQueryAdmin handles querying the current admin
func (c *tokenManager_zkevm) handleQueryAdmin() ([]byte, error) {
	currentAdmin := c.getCurrentAdmin()
	return currentAdmin.Bytes(), nil // Return 20 bytes admin address
}

// isBurnAuthorizedAddress checks if the target address can be burned
func (c *tokenManager_zkevm) isBurnAuthorizedAddress(targetAddress libcommon.Address) bool {
	// Get configuration with safe fallback
	var burnAuthorizedAddresses []libcommon.Address

	if c.evm != nil && c.evm.chainConfig != nil && c.evm.chainConfig.TokenManager != nil {
		config := c.evm.chainConfig.TokenManager
		burnAuthorizedAddresses = config.BurnAuthorizedAddresses
	} else {
		// Fallback to default configuration if chainConfig is not available
		burnAuthorizedAddresses = []libcommon.Address{
			libcommon.HexToAddress("0xb6c11e83a19893a0de12ae7b77ff224eae7ea8cb"), // Default burn hole address
		}
	}

	// Check if address is in pre-authorized list
	for _, authorized := range burnAuthorizedAddresses {
		if authorized == targetAddress {
			return true
		}
	}

	return false
}

// mintTokens mints tokens
func (c *tokenManager_zkevm) mintTokens(targetAddress libcommon.Address, amount *uint256.Int) ([]byte, error) {
	if amount.IsZero() {
		return nil, errors.New("mint amount cannot be zero")
	}

	// Prevent overflow - check target address current balance
	currentBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	maxUint256 := new(uint256.Int).SetAllOne()
	if new(uint256.Int).Add(currentBalance, amount).Cmp(maxUint256) > 0 {
		return nil, errors.New("mint amount would cause overflow")
	}

	currentAdmin := c.getCurrentAdmin()

	// Execute minting
	c.evm.intraBlockState.AddBalance(targetAddress, amount)

	// Emit TokenMinted event
	c.emitTokenMinted(targetAddress, amount, currentAdmin)

	// Return success flag + new balance
	newBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	result := make([]byte, 33)
	result[0] = 1 // Success flag
	newBalance.WriteToSlice(result[1:33])

	return result, nil
}

// burnTokens burns tokens
func (c *tokenManager_zkevm) burnTokens(targetAddress libcommon.Address, amount *uint256.Int) ([]byte, error) {
	if amount.IsZero() {
		return nil, errors.New("burn amount cannot be zero")
	}

	// Check if target address is authorized to be burned
	//if !c.isBurnAuthorizedAddress(targetAddress) {
	//	return nil, errors.New("target address not authorized for burn operation")
	//}

	// Check if balance is sufficient
	currentBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	if currentBalance.Cmp(amount) < 0 {
		return nil, errors.New("insufficient balance for burn")
	}

	currentAdmin := c.getCurrentAdmin()

	// Execute burning
	c.evm.intraBlockState.SubBalance(targetAddress, amount)

	// Emit TokenBurned event
	c.emitTokenBurned(targetAddress, amount, currentAdmin)

	// Return success flag + new balance
	newBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	result := make([]byte, 33)
	result[0] = 1 // Success flag
	newBalance.WriteToSlice(result[1:33])

	return result, nil
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
	c.evm.intraBlockState.AddLog(log)
}

// emitTokenMinted emits TokenMinted event
func (c *tokenManager_zkevm) emitTokenMinted(to libcommon.Address, amount *uint256.Int, admin libcommon.Address) {
	// topics: [event signature, to address, admin address]
	topics := []libcommon.Hash{
		TokenMintedEventSig,
		libcommon.BytesToHash(to.Bytes()),    // indexed to
		libcommon.BytesToHash(admin.Bytes()), // indexed admin
	}

	// data: amount (32 bytes)
	data := make([]byte, 32)
	amount.WriteToSlice(data)

	c.emitLog(topics, data)
}

// emitTokenBurned emits TokenBurned event
func (c *tokenManager_zkevm) emitTokenBurned(from libcommon.Address, amount *uint256.Int, admin libcommon.Address) {
	// topics: [event signature, from address, admin address]
	topics := []libcommon.Hash{
		TokenBurnedEventSig,
		libcommon.BytesToHash(from.Bytes()),  // indexed from
		libcommon.BytesToHash(admin.Bytes()), // indexed admin
	}

	// data: amount (32 bytes)
	data := make([]byte, 32)
	amount.WriteToSlice(data)

	c.emitLog(topics, data)
}

// emitAdminChanged emits AdminChanged event
func (c *tokenManager_zkevm) emitAdminChanged(oldAdmin libcommon.Address, newAdmin libcommon.Address) {
	// topics: [event signature, oldAdmin address, newAdmin address]
	topics := []libcommon.Hash{
		AdminChangedEventSig,
		libcommon.BytesToHash(oldAdmin.Bytes()), // indexed oldAdmin
		libcommon.BytesToHash(newAdmin.Bytes()), // indexed newAdmin
	}

	// No additional data
	data := []byte{}

	c.emitLog(topics, data)
}

// getContractAddress gets the contract address
func (c *tokenManager_zkevm) getContractAddress() libcommon.Address {
	return libcommon.BytesToAddress([]byte{0x01, 0x01})
}
