package vm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/params"
)

var CONFIG_CONTRACT_ADDRESS = libcommon.HexToAddress("0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab")

// Token Manager precompile address
var TOKEN_MANAGER_ADDRESS = libcommon.HexToAddress("0x0000000000000000000000000000000000000101")

// Operation codes definition
const (
	TOKEN_MINT_OP  = 0x01 // Token minting operation
	TOKEN_BURN_OP  = 0x02 // Token burning operation
	QUERY_ADMIN_OP = 0x20 // Query current admin
)

// tokenManager_zkevm precompile contract
type tokenManager_zkevm struct {
	evm     *EVM
	enabled bool
}

// Event signatures for logging
var (
	// TokenMinted(address indexed to, uint256 amount, address indexed admin)
	TokenMintedEventSig = libcommon.HexToHash("0xab8530f87dc9b59234c4623bf917212bb2536d647574c8e7e5da92c2ede0c9f8")

	// TokenBurned(address indexed from, uint256 amount, address indexed admin)
	TokenBurnedEventSig = libcommon.HexToHash("0xcc16f5dbb4873280815c1ee09dbd06736cffcc184412cf7a71a0fdb75d397ca5")
)

// RequiredGas returns the required gas for Token Manager operations
func (c *tokenManager_zkevm) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return 0
	}

	operation := input[0]
	switch operation {
	case TOKEN_MINT_OP:
		return 0 // Mint operations are gas-free
	case TOKEN_BURN_OP:
		return params.CallValueTransferGas // Burn operations consume gas
	case QUERY_ADMIN_OP:
		return params.ColdSloadCostEIP2929 // Query operations have low gas cost
	default:
		return 0
	}
}

// SetCounterCollector sets the counter collector (required by zkEVM interface)
func (c *tokenManager_zkevm) SetCounterCollector(cc *CounterCollector) {
	// Token Manager doesn't use counter collector
}

// SetOutputLength sets the output length (required by zkEVM interface)
func (c *tokenManager_zkevm) SetOutputLength(outLength int) {
	// Token Manager doesn't use output length setting
}

// SetEVM sets the EVM reference
func (c *tokenManager_zkevm) SetEVM(evm *EVM) {
	c.evm = evm
}

// Run executes the precompile contract
func (c *tokenManager_zkevm) Run(input []byte) ([]byte, error) {
	if !c.enabled {
		return []byte{}, ErrUnsupportedPrecompile
	}

	// Check if configuration contract is deployed
	if !c.isConfigContractDeployed() {
		return []byte{}, errors.New("Token Manager configuration contract not deployed yet")
	}

	// Quick activation check first (single StaticCall)
	if !c.isTokenManagerActive() {
		return []byte{}, errors.New("Token Manager is not activated")
	}

	if len(input) == 0 {
		return []byte{}, errors.New("empty input")
	}

	operation := input[0]
	data := input[1:]

	switch operation {
	case TOKEN_MINT_OP, TOKEN_BURN_OP:
		return c.handleTokenOperation(operation, data)
	case QUERY_ADMIN_OP:
		return c.handleQueryAdmin()
	default:
		return nil, errors.New("invalid operation")
	}
}

// isConfigContractDeployed checks if the configuration contract is deployed
func (c *tokenManager_zkevm) isConfigContractDeployed() bool {
	// Check if there's code at the configuration contract address
	code := c.evm.IntraBlockState().GetCode(CONFIG_CONTRACT_ADDRESS)
	return len(code) > 0
}

// isTokenManagerActive checks if Token Manager is active (single StaticCall)
// Note: This assumes the config contract is already deployed (checked by caller)
func (c *tokenManager_zkevm) isTokenManagerActive() bool {
	isActiveData := crypto.Keccak256([]byte("isActive()"))[:4]
	isActiveResult, _, err := c.evm.StaticCall_zkEvm(AccountRef(c.evm.TxContext.Origin), CONFIG_CONTRACT_ADDRESS, isActiveData, 10000, 32)
	if err != nil {
		return false
	}

	if len(isActiveResult) < 32 {
		return false
	}

	return new(big.Int).SetBytes(isActiveResult[len(isActiveResult)-32:]).Uint64() != 0
}

// getAdminAddress gets the current admin address (single StaticCall)
// Note: This assumes the config contract is already deployed (checked by caller)
func (c *tokenManager_zkevm) getAdminAddress() (libcommon.Address, error) {
	getAdminData := crypto.Keccak256([]byte("getAdmin()"))[:4]
	adminResult, _, err := c.evm.StaticCall_zkEvm(AccountRef(c.evm.TxContext.Origin), CONFIG_CONTRACT_ADDRESS, getAdminData, 10000, 32)
	if err != nil {
		return libcommon.Address{}, fmt.Errorf("failed to get admin address: %v", err)
	}

	if len(adminResult) < 32 {
		return libcommon.Address{}, errors.New("invalid admin response")
	}

	// Parse admin address (last 20 bytes of the 32-byte response)
	adminAddress := libcommon.BytesToAddress(adminResult[12:32])
	return adminAddress, nil
}

// checkBurnWhitelist checks if an address is allowed to burn tokens (single StaticCall)
// Note: This assumes the config contract is already deployed (checked by caller)
func (c *tokenManager_zkevm) checkBurnWhitelist(addr libcommon.Address) (bool, error) {
	// Call isBurnAllowed(address) function
	isBurnAllowedData := crypto.Keccak256([]byte("isBurnAllowed(address)"))[:4]

	// Encode the address parameter (32-byte padded)
	addressParam := make([]byte, 32)
	copy(addressParam[12:], addr.Bytes()) // Address goes in the last 20 bytes

	// Combine function selector and parameter
	callData := append(isBurnAllowedData, addressParam...)

	result, _, err := c.evm.StaticCall_zkEvm(AccountRef(c.evm.TxContext.Origin), CONFIG_CONTRACT_ADDRESS, callData, 10000, 32)
	if err != nil {
		return false, fmt.Errorf("failed to check burn whitelist: %v", err)
	}

	if len(result) < 32 {
		return false, errors.New("invalid burn whitelist response")
	}

	// Parse boolean result
	return new(big.Int).SetBytes(result[len(result)-32:]).Uint64() != 0, nil
}

// emitTokenEvent emits token operation events (mint/burn)
func (c *tokenManager_zkevm) emitTokenEvent(eventSig libcommon.Hash, targetAddress libcommon.Address, amount *uint256.Int, admin libcommon.Address) {
	c.evm.IntraBlockState().AddLog(&types.Log{
		Address: TOKEN_MANAGER_ADDRESS,
		Topics: []libcommon.Hash{
			eventSig,
			libcommon.BytesToHash(targetAddress.Bytes()),
			libcommon.BytesToHash(admin.Bytes()),
		},
		Data: amount.Bytes(),
	})
}

// handleTokenOperation handles token operations
func (c *tokenManager_zkevm) handleTokenOperation(operation byte, data []byte) ([]byte, error) {
	// Get admin address (single StaticCall)
	admin, err := c.getAdminAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get admin: %v", err)
	}

	if admin == (libcommon.Address{}) {
		return nil, errors.New("no admin configured")
	}

	// Check if caller is the admin (owner)
	caller := c.evm.TxContext.Origin
	if caller != admin {
		return nil, errors.New("unauthorized: only admin can perform token operations")
	}

	// Simple operation (64 bytes: 32 bytes address + 32 bytes amount)
	if len(data) < 64 {
		return nil, errors.New("invalid token operation data")
	}

	// Address is 32-byte encoded (first 12 bytes are padding, last 20 bytes are the actual address)
	targetAddress := libcommon.BytesToAddress(data[12:32])
	amount := new(uint256.Int).SetBytes(data[32:64])

	switch operation {
	case TOKEN_MINT_OP:
		return c.mintTokens(targetAddress, amount, caller)
	case TOKEN_BURN_OP:
		// Check burn whitelist before burning (single StaticCall)
		isAllowed, err := c.checkBurnWhitelist(targetAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to check burn whitelist: %v", err)
		}
		if !isAllowed {
			return nil, fmt.Errorf("address %s is not in burn whitelist", targetAddress.Hex())
		}
		return c.burnTokens(targetAddress, amount, caller)
	}

	return nil, errors.New("invalid token operation")
}

// handleQueryAdmin handles querying the current admin
func (c *tokenManager_zkevm) handleQueryAdmin() ([]byte, error) {
	admin, err := c.getAdminAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get admin: %v", err)
	}

	return admin.Bytes(), nil
}

// mintTokens handles token minting
func (c *tokenManager_zkevm) mintTokens(to libcommon.Address, amount *uint256.Int, admin libcommon.Address) ([]byte, error) {
	// Check amount validity
	if amount == nil || amount.IsZero() {
		return nil, errors.New("invalid mint amount")
	}

	// Add the minted amount directly
	c.evm.IntraBlockState().AddBalance(to, amount)

	// Emit mint event
	c.emitTokenEvent(TokenMintedEventSig, to, amount, admin)

	return []byte{}, nil
}

// burnTokens handles token burning with crash prevention and whitelist check
func (c *tokenManager_zkevm) burnTokens(from libcommon.Address, amount *uint256.Int, admin libcommon.Address) ([]byte, error) {
	// Check amount validity
	if amount == nil || amount.IsZero() {
		return nil, errors.New("invalid burn amount")
	}

	// Get current balance
	currentBalance := c.evm.IntraBlockState().GetBalance(from)

	// CRITICAL: Prevent full burn to avoid node crashes
	// This protection mechanism is essential for chain stability
	if currentBalance.Cmp(amount) <= 0 {
		return nil, errors.New("insufficient balance for burn: cannot burn entire balance")
	}

	// Subtract the burned amount directly
	c.evm.IntraBlockState().SubBalance(from, amount)

	// Emit burn event
	c.emitTokenEvent(TokenBurnedEventSig, from, amount, admin)

	return []byte{}, nil
}
