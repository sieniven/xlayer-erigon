package vm

import (
	"errors"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/params"
)

// Default admin addresses (hardcoded for consistency)
var ADMIN_ADDRESSES = []libcommon.Address{
	libcommon.HexToAddress("0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15"), // Admin 1
	libcommon.HexToAddress("0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"), // Admin 2
	libcommon.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"), // Admin 3
}

// Minimum required signatures (configurable for testing)
var MIN_SIGNATURES = uint64(2) // Require 2 out of 3 signatures

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

	QUERY_ADMIN_OP = 0x20 // Query current admin
)

// Event signatures for logging
var (
	// TokenMinted(address indexed to, uint256 amount, address indexed admin)
	TokenMintedEventSig = libcommon.HexToHash("0xab8530f87dc9b59234c4623bf917212bb2536d647574c8e7e5da92c2ede0c9f8")

	// TokenBurned(address indexed from, uint256 amount, address indexed admin)
	TokenBurnedEventSig = libcommon.HexToHash("0xcc16f5dbb4873280815c1ee09dbd06736cffcc184412cf7a71a0fdb75d397ca5")
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

	operation := input[0]
	switch operation {
	case TOKEN_MINT_OP:
		return params.TokenMintGas // Free mint operation
	case TOKEN_BURN_OP:
		return params.TokenBurnGas
	case QUERY_ADMIN_OP:
		return params.TokenQueryGas
	default:
		return 0
	}
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
	if !c.enabled {
		return []byte{}, ErrUnsupportedPrecompile
	}

	// If Token Manager is not yet active, behave like an empty account call
	// This ensures consensus compatibility with nodes that don't have Token Manager
	if !c.isTokenManagerActive() {
		return []byte{}, errors.New("token Manager not activated: missing configuration or activation block not reached")
	}

	if len(input) < 1 {
		return nil, errors.New("invalid input: missing operation code")
	}

	operation := input[0]
	switch operation {
	case TOKEN_MINT_OP, TOKEN_BURN_OP:
		return c.handleMultiSigOperation(input)
	case QUERY_ADMIN_OP:
		return c.handleQueryAdmins()
	default:
		return nil, errors.New("unsupported operation")
	}
}

// handleMultiSigOperation handles multi-signature operations (mint/burn)
func (c *tokenManager_zkevm) handleMultiSigOperation(input []byte) ([]byte, error) {
	// Parse multi-signature data: [operation:1][target:32][amount:32][nonce:8][sigCount:1][signatures...]
	// Each signature: [v:1][r:32][s:32] = 65 bytes per signature

	if len(input) < 74 { // 1+32+32+8+1 = minimum without signatures
		return nil, errors.New("invalid multi-sig data: insufficient length")
	}

	// Parse basic data
	operation := input[0]
	targetAddr := libcommon.BytesToAddress(input[1:33])
	amount := uint256.NewInt(0).SetBytes(input[33:65])
	nonce := uint256.NewInt(0).SetBytes(input[65:73]).Uint64()
	sigCount := input[73]

	// Validate expected length
	expectedLen := 74 + int(sigCount)*65 // 65 bytes per signature
	if len(input) < expectedLen {
		return nil, errors.New("invalid multi-sig data: signature length mismatch")
	}

	// Extract signatures
	signatures := make([][]byte, sigCount)
	offset := 74
	for i := 0; i < int(sigCount); i++ {
		signatures[i] = input[offset : offset+65]
		offset += 65
	}

	// Verify multi-signature
	if !c.verifyMultiSignatures(operation, targetAddr, amount, nonce, signatures) {
		return nil, errors.New("insufficient valid signatures")
	}

	// Get first valid signer for event logging
	firstSigner := c.getFirstValidSigner(operation, targetAddr, amount, nonce, signatures)

	// Execute the operation
	switch operation {
	case TOKEN_MINT_OP:
		return c.mintTokens(targetAddr, amount, []libcommon.Address{firstSigner})
	case TOKEN_BURN_OP:
		return c.burnTokens(targetAddr, amount, []libcommon.Address{firstSigner})
	default:
		return nil, errors.New("invalid operation")
	}
}

// verifyMultiSignatures verifies if the multi-signature meets requirements
func (c *tokenManager_zkevm) verifyMultiSignatures(operation byte, target libcommon.Address, amount *uint256.Int, nonce uint64, signatures [][]byte) bool {
	minSigs := c.getMinSignatures()

	if len(signatures) < int(minSigs) {
		return false
	}

	// Create message hash for signature verification
	msgHash := c.createMessageHash(operation, target, amount, nonce)

	validSigs := 0
	usedAdmins := make(map[libcommon.Address]bool)

	for _, signature := range signatures {
		if len(signature) != 65 {
			continue // Skip invalid signature format
		}

		// Recover signer address from signature
		signer, err := c.recoverSigner(msgHash, signature)
		if err != nil {
			continue // Skip if can't recover
		}

		// Check if signer is a valid admin and not already used
		if c.isValidAdmin(signer) && !usedAdmins[signer] {
			validSigs++
			usedAdmins[signer] = true
		}
	}

	return validSigs >= int(minSigs)
}

// recoverSigner recovers the signer address from signature
func (c *tokenManager_zkevm) recoverSigner(msgHash []byte, signature []byte) (libcommon.Address, error) {
	if len(signature) != 65 {
		return libcommon.Address{}, errors.New("invalid signature length")
	}

	// Recover public key from signature
	pubkey, err := crypto.SigToPub(msgHash, signature)
	if err != nil {
		return libcommon.Address{}, err
	}

	// Get address from public key
	return crypto.PubkeyToAddress(*pubkey), nil
}

// getFirstValidSigner gets the first valid signer for event logging
func (c *tokenManager_zkevm) getFirstValidSigner(operation byte, target libcommon.Address, amount *uint256.Int, nonce uint64, signatures [][]byte) libcommon.Address {
	msgHash := c.createMessageHash(operation, target, amount, nonce)

	for _, signature := range signatures {
		if len(signature) != 65 {
			continue
		}

		signer, err := c.recoverSigner(msgHash, signature)
		if err != nil {
			continue
		}

		if c.isValidAdmin(signer) {
			return signer
		}
	}

	// Fallback to first admin if no valid signer found
	if len(ADMIN_ADDRESSES) > 0 {
		return ADMIN_ADDRESSES[0]
	}
	return libcommon.Address{}
}

// createMessageHash creates a hash for signature verification
func (c *tokenManager_zkevm) createMessageHash(operation byte, target libcommon.Address, amount *uint256.Int, nonce uint64) []byte {
	// This function is no longer used in simplified implementation
	// Kept for future multi-signature implementation
	data := make([]byte, 0, 1+32+32+8)
	data = append(data, operation)
	data = append(data, target.Bytes()...)
	data = append(data, amount.PaddedBytes(32)...)
	nonceBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		nonceBytes[7-i] = byte(nonce >> (i * 8))
	}
	data = append(data, nonceBytes...)

	return crypto.Keccak256(data)
}

// getAdminAddresses gets admin addresses from config or default
func (c *tokenManager_zkevm) getAdminAddresses() []libcommon.Address {
	return ADMIN_ADDRESSES
}

// getMinSignatures returns hardcoded minimum required signatures
func (c *tokenManager_zkevm) getMinSignatures() uint64 {
	return MIN_SIGNATURES
}

// isValidAdmin checks if an address is a valid admin
func (c *tokenManager_zkevm) isValidAdmin(addr libcommon.Address) bool {
	for _, admin := range ADMIN_ADDRESSES {
		if addr == admin {
			return true
		}
	}
	return false
}

// handleQueryAdmins returns current admin addresses
func (c *tokenManager_zkevm) handleQueryAdmins() ([]byte, error) {
	adminAddrs := c.getAdminAddresses()
	minSigs := c.getMinSignatures()

	// Return format: [adminCount:1][minSigs:8][admin1:32][admin2:32]...
	result := make([]byte, 0, 9+len(adminAddrs)*32)
	result = append(result, byte(len(adminAddrs)))

	minSigBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		minSigBytes[7-i] = byte(minSigs >> (i * 8))
	}
	result = append(result, minSigBytes...)

	for _, addr := range adminAddrs {
		paddedAddr := make([]byte, 32)
		copy(paddedAddr[12:], addr.Bytes())
		result = append(result, paddedAddr...)
	}

	return result, nil
}

// mintTokens mints tokens (signature verification done in handleMultiSigOperation)
func (c *tokenManager_zkevm) mintTokens(targetAddress libcommon.Address, amount *uint256.Int, signers []libcommon.Address) ([]byte, error) {
	if amount.IsZero() {
		return nil, errors.New("mint amount cannot be zero")
	}

	// Prevent overflow - check target address current balance
	currentBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	maxUint256 := new(uint256.Int).SetAllOne()
	if new(uint256.Int).Add(currentBalance, amount).Cmp(maxUint256) > 0 {
		return nil, errors.New("mint amount would cause overflow")
	}

	// Execute minting
	c.evm.intraBlockState.AddBalance(targetAddress, amount)

	// Emit TokenMinted event
	c.emitTokenMinted(targetAddress, amount, signers[0]) // Use the first signer as admin for event

	// Return success flag + new balance
	newBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	result := make([]byte, 33)
	result[0] = 1 // Success flag
	newBalance.WriteToSlice(result[1:33])

	return result, nil
}

// burnTokens burns tokens (signature verification done in handleMultiSigOperation)
func (c *tokenManager_zkevm) burnTokens(targetAddress libcommon.Address, amount *uint256.Int, signers []libcommon.Address) ([]byte, error) {
	if amount.IsZero() {
		return nil, errors.New("burn amount cannot be zero")
	}

	// Check if target address is authorized to be burned
	//if !c.isBurnAuthorizedAddress(targetAddress) {
	//	return nil, errors.New("target address not authorized for burn operation")
	//}

	// Check if balance is sufficient
	currentBalance := c.evm.intraBlockState.GetBalance(targetAddress)
	if currentBalance.Cmp(amount) <= 0 { // cannot burn all gas token, or it will throw nil panic in sequencer
		return nil, errors.New("insufficient balance for burn")
	}

	// Execute burning
	c.evm.intraBlockState.SubBalance(targetAddress, amount)

	// Emit TokenBurned event
	c.emitTokenBurned(targetAddress, amount, signers[0]) // Use the first signer as admin for event

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

// getContractAddress gets the contract address
func (c *tokenManager_zkevm) getContractAddress() libcommon.Address {
	return libcommon.BytesToAddress([]byte{0x01, 0x01})
}

// isTokenManagerActive checks if Token Manager is active based on current block height
func (c *tokenManager_zkevm) isTokenManagerActive() bool {
	// Always active for hardcoded implementation
	return true
}

// isBurnAuthorizedAddress checks if an address is authorized for burn operations
func (c *tokenManager_zkevm) isBurnAuthorizedAddress(address libcommon.Address) bool {
	// Allow any address for hardcoded implementation (commented out in burnTokens anyway)
	return true
}
