#!/bin/bash

# Token Manager Contracts Deployment Script
# This script deploys the Token Manager system using standard CREATE deployment
set -e

echo "🚀 Token Manager Deployment Script"
echo "=================================="
echo "📋 Features: Mint/Burn + OpenZeppelin Security"
echo ""

# Configuration
PRIVATE_KEY="${PRIVATE_KEY:-0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9}"
RPC_URL="${RPC_URL:-http://localhost:8123}"
OWNER_ADMIN="${OWNER_ADMIN:-0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15}"
PROXY_ADMIN="${PROXY_ADMIN:-$OWNER_ADMIN}"

echo "📊 Deployment Configuration:"
echo "  RPC URL: $RPC_URL"
echo "  Owner/Admin: $OWNER_ADMIN" 
echo "  Proxy Admin: $PROXY_ADMIN"
echo ""

# Check network connectivity
echo "🔍 Checking network connectivity..."
if ! cast chain-id --rpc-url $RPC_URL > /dev/null 2>&1; then
    echo "❌ Error: Cannot connect to RPC endpoint $RPC_URL"
    exit 1
fi

CHAIN_ID=$(cast chain-id --rpc-url $RPC_URL)
BLOCK_NUMBER=$(cast block-number --rpc-url $RPC_URL)
echo "✅ Connected to Chain ID: $CHAIN_ID, Block: $BLOCK_NUMBER"
echo ""

# Check deployer balance
DEPLOYER=$(cast wallet address --private-key $PRIVATE_KEY)
BALANCE=$(cast balance $DEPLOYER --rpc-url $RPC_URL)
echo "👛 Deployer: $DEPLOYER"
echo "💰 Balance: $BALANCE wei"

if [ "$BALANCE" = "0" ]; then
    echo "❌ Error: Deployer has no balance"
    exit 1
fi
echo ""

# Compile contracts
echo "🔧 Compiling contracts..."
cd contracts

# Check if solc is available
if ! command -v solc &> /dev/null; then
    echo "❌ Error: solc compiler not found. Please install solidity compiler."
    exit 1
fi

# Compile TokenManagerV1
echo "  Compiling TokenManagerV1..."
solc --bin --evm-version paris TokenManagerV1.sol -o . --overwrite --base-path . --include-path node_modules/ > /dev/null 2>&1

if [ ! -f "TokenManagerV1.bin" ]; then
    echo "❌ Error: TokenManagerV1 compilation failed"
    exit 1
fi

# Compile TokenManagerProxy  
echo "  Compiling TokenManagerProxy..."
solc --bin --evm-version paris TokenManagerProxy.sol -o . --overwrite --base-path . --include-path node_modules/ > /dev/null 2>&1

if [ ! -f "TokenManagerProxy.bin" ]; then
    echo "❌ Error: TokenManagerProxy compilation failed"
    exit 1
fi

cd ..
echo "✅ Contracts compiled successfully"
echo ""

# Read contract bytecode
IMPL_BYTECODE="0x$(cat contracts/TokenManagerV1.bin)"
PROXY_BYTECODE="0x$(cat contracts/TokenManagerProxy.bin)"

# Validate bytecode
echo "🔍 Validating compiled bytecode..."
if [ ${#IMPL_BYTECODE} -le 1000 ]; then
    echo "❌ Error: Implementation bytecode too short (${#IMPL_BYTECODE} chars, expected >1000)"
    echo "   This usually means compilation failed or contract is too simple"
    exit 1
fi

if [ ${#PROXY_BYTECODE} -le 1000 ]; then
    echo "❌ Error: Proxy bytecode too short (${#PROXY_BYTECODE} chars, expected >1000)"
    echo "   This usually means compilation failed or contract is too simple"
    exit 1
fi



echo "📦 Bytecode ready:"
echo "  Implementation: ${#IMPL_BYTECODE} characters"
echo "  Proxy: ${#PROXY_BYTECODE} characters" 
echo ""

# Deploy Implementation Contract
echo "📋 Step 1: Deploying Implementation Contract..."
echo "  Contract: TokenManagerV1"
echo "  Method: Standard CREATE deployment"

IMPL_TX=$(cast send --private-key $PRIVATE_KEY --rpc-url $RPC_URL --legacy --create $IMPL_BYTECODE 2>/dev/null)

if [ $? -ne 0 ]; then
    echo "❌ Error: Implementation deployment transaction failed"
    exit 1
fi

echo "  Transaction: $IMPL_TX"

# Wait for transaction confirmation
echo "  Waiting for confirmation..."
sleep 3

# Get implementation address
IMPL_ADDRESS=$(cast receipt $IMPL_TX --field contractAddress --rpc-url $RPC_URL 2>/dev/null)

if [ -z "$IMPL_ADDRESS" ] || [ "$IMPL_ADDRESS" = "null" ]; then
    echo "❌ Error: Failed to get implementation contract address"
    exit 1
fi

# Verify implementation deployment
IMPL_CODE=$(cast code $IMPL_ADDRESS --rpc-url $RPC_URL)
if [ ${#IMPL_CODE} -le 2 ]; then
    echo "❌ Error: Implementation contract not deployed (no code at address)"
    exit 1
fi

echo "✅ Implementation deployed at: $IMPL_ADDRESS"
echo ""

# Deploy Proxy Contract  
echo "📋 Step 2: Deploying Proxy Contract..."
echo "  Contract: TokenManagerProxy"
echo "  Implementation: $IMPL_ADDRESS"
echo "  Admin: $PROXY_ADMIN"

# Prepare proxy constructor data
PROXY_CONSTRUCTOR=$(cast abi-encode "constructor(address,address,bytes)" $IMPL_ADDRESS $PROXY_ADMIN "0x")
PROXY_DEPLOY_DATA="${PROXY_BYTECODE}${PROXY_CONSTRUCTOR:2}"

PROXY_TX=$(cast send --private-key $PRIVATE_KEY --rpc-url $RPC_URL --legacy --create "$PROXY_DEPLOY_DATA" 2>/dev/null)

if [ $? -ne 0 ]; then
    echo "❌ Error: Proxy deployment transaction failed"
    exit 1
fi

echo "  Transaction: $PROXY_TX"

# Wait for transaction confirmation
echo "  Waiting for confirmation..."
sleep 3

# Get proxy address
PROXY_ADDRESS=$(cast receipt $PROXY_TX --field contractAddress --rpc-url $RPC_URL 2>/dev/null)

if [ -z "$PROXY_ADDRESS" ] || [ "$PROXY_ADDRESS" = "null" ]; then
    echo "❌ Error: Failed to get proxy contract address"
    exit 1
fi

# Verify proxy deployment
PROXY_CODE=$(cast code $PROXY_ADDRESS --rpc-url $RPC_URL)
if [ ${#PROXY_CODE} -le 2 ]; then
    echo "❌ Error: Proxy contract not deployed (no code at address)"
    exit 1
fi

echo "✅ Proxy deployed at: $PROXY_ADDRESS"
echo ""

# Initialize the proxy contract
echo "📋 Step 3: Initializing Contract..."
echo "  Calling initialize() with owner: $OWNER_ADMIN"

INIT_DATA=$(cast calldata "initialize(address)" $OWNER_ADMIN)
INIT_TX=$(cast send --private-key $PRIVATE_KEY --rpc-url $RPC_URL --to $PROXY_ADDRESS $INIT_DATA 2>/dev/null)

if [ $? -ne 0 ]; then
    echo "❌ Error: Initialization transaction failed"
    exit 1
fi

echo "  Transaction: $INIT_TX"

# Wait for transaction confirmation
echo "  Waiting for confirmation..."
sleep 3

echo "✅ Contract initialized successfully"
echo ""

# Verification
echo "🔍 Step 4: Verifying Deployment..."

# Check owner
CURRENT_OWNER=$(cast call --rpc-url $RPC_URL $PROXY_ADDRESS "owner()" 2>/dev/null)
EXPECTED_OWNER=$(cast to-check-sum-address $OWNER_ADMIN)

if [ "$CURRENT_OWNER" != "$EXPECTED_OWNER" ]; then
    echo "❌ Error: Owner verification failed"
    echo "  Expected: $EXPECTED_OWNER"
    echo "  Actual: $CURRENT_OWNER"
    exit 1
fi

# Check contract version
VERSION_RAW=$(cast call --rpc-url $RPC_URL $PROXY_ADDRESS "VERSION()" 2>/dev/null)
VERSION=$(cast to-ascii $VERSION_RAW 2>/dev/null || echo "Unable to decode")

# Check activation status
IS_ACTIVE_RAW=$(cast call --rpc-url $RPC_URL $PROXY_ADDRESS "isActive()" 2>/dev/null)
IS_ACTIVE=$([ "$IS_ACTIVE_RAW" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "Active" || echo "Inactive")

echo "✅ Verification completed:"
echo "  Owner: $CURRENT_OWNER"
echo "  Version: $VERSION"
echo "  Status: $IS_ACTIVE"
echo ""



# Deployment Summary
echo "🎉 Deployment Summary"
echo "===================="
echo ""
echo "📋 Deployed Contracts:"
echo "  Implementation: $IMPL_ADDRESS"
echo "  Proxy (Main):   $PROXY_ADDRESS"
echo ""
echo "👑 Access Control:"
echo "  Contract Owner: $CURRENT_OWNER"
echo "  Proxy Admin:    $PROXY_ADMIN"
echo ""
echo "⚙️  Configuration:"
echo "  Status: $IS_ACTIVE"
echo "  Version: $VERSION"
echo ""
echo "📝 Next Steps:"
echo "  1. Update CONFIG_CONTRACT_MANAGER_ADDRESS in core/vm/contracts_mint_burn.go:"
echo "     CONFIG_CONTRACT_MANAGER_ADDRESS = common.HexToAddress(\"$PROXY_ADDRESS\")"
echo ""
echo "  2. Recompile and restart the node"
echo ""  
echo "  3. Activate the Token Manager (if needed):"
echo "     cast send --private-key \$ADMIN_KEY --rpc-url $RPC_URL \\"
echo "       --to $PROXY_ADDRESS \"setActivationBlock(uint256)\" 0"
echo ""
echo "  4. Add addresses to burn whitelist (if needed):"
echo "     cast send --private-key \$ADMIN_KEY --rpc-url $RPC_URL \\"
echo "       --to $PROXY_ADDRESS \"addBurnWhitelist(address)\" \$TARGET_ADDRESS"
echo ""
echo "✅ Token Manager deployment completed successfully!"