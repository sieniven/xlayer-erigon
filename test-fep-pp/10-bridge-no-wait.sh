#!/bin/bash

# Strict mode: exit on command failure or undefined variable
set -eu
# set -x

# =============================================================================
# Configuration
# =============================================================================
BRIDGE_ADDRESS="0x3a277Fa4E78cc1266F32E26c467F99A8eAEfF7c3"
ACCOUNT="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534" 
PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
BRIDGE_VALUE_BIG="1000000000000000000"  # 1 ETH in wei
BRIDGE_VALUE_SMALL="100000000000000000"  # 0.1 ETH in wei

L1_ETH_ADDRESS="0x0000000000000000000000000000000000000000"
L2_WETH="0x17a2a2e444a7f3446877d1b71eaa2b2ae7533baf"

# =============================================================================
# RPC Endpoint Configuration
# =============================================================================
L1RPC=http://127.0.0.1:8545
L2RPC=http://127.0.0.1:8123
BRIDGE_SERVICE1=http://127.0.0.1:8080

# Get Global Exit Root Manager address
GER_MGR=$(cast call "$BRIDGE_ADDRESS" "globalExitRootManager()(address)" --rpc-url "$L1RPC")
L2GER_MGR1=$(cast call "$BRIDGE_ADDRESS" "globalExitRootManager()(address)" --rpc-url "$L2RPC")
echo "GlobalExitRootManager Address:"
echo "  L1   ==> $GER_MGR"
echo "  L2   ==> $L2GER_MGR1"

# Get initial Global Exit Root
GER=$(cast call "$GER_MGR" "getLastGlobalExitRoot()" --rpc-url "$L1RPC")
echo "Initial GER on L1: $GER"

# =============================================================================
# Bridge from L1 to L2
# =============================================================================

# Check balance before bridging
L2_BALANCE_BEFORE_BRIDGE=$(cast call "$L2_WETH" "balanceOf(address)(uint256)" "$ACCOUNT" --rpc-url "$L2RPC" | awk '{print $1}')

cast send \
    --legacy \
    --rpc-url $L1RPC \
    --private-key $PRIVATE_KEY \
    --value $BRIDGE_VALUE_BIG \
    $BRIDGE_ADDRESS \
    'function bridgeAsset(uint32 destinationNetwork, address destinationAddress, uint256 amount, address token, bool forceUpdateGlobalExitRoot, bytes permitData) returns()' \
    1 $ACCOUNT $BRIDGE_VALUE_BIG $L1_ETH_ADDRESS true 0x

# Wait for GER update on L1
echo "Waiting for GER to be updated on L1..."

# Wait for GER to sync to L2
echo "Waiting for GER to sync to L2..."
sleep 1

# Wait for assets to be claimed automatically by sponsor
echo "Waiting for assets to be claimed by sponsor..."
sleep 1
    
# sleep 10
# done

# Check balance after bridging
L2_BALANCE_AFTER_BRIDGE=$(cast balance "$ACCOUNT" --rpc-url "$L2RPC")
echo "Balance on L2(ETH):"
echo "  Before bridge = $L2_BALANCE_BEFORE_BRIDGE"
echo "  After bridge  = $L2_BALANCE_AFTER_BRIDGE"

# =============================================================================
# Bridge from L2 to L1
# =============================================================================
echo -e "\n========== Bridging Assets: L2 -> L1 BRIDGE_VALUE_SMALL =========="

# Initiate bridging transaction
TX_HASH=$(cast send \
    --legacy \
    --private-key $PRIVATE_KEY \
    --rpc-url $L2RPC \
    --json \
    $BRIDGE_ADDRESS \
    'function bridgeAsset(uint32 destinationNetwork, address destinationAddress, uint256 amount, address token, bool forceUpdateGlobalExitRoot, bytes permitData) returns()' \
    0 $ACCOUNT $BRIDGE_VALUE_SMALL $L2_WETH true "0x" \
    | jq -r ' .transactionHash')
echo "Bridge transaction hash: $TX_HASH"

# Wait for GER update on L1
echo "Waiting for GER to be updated on L1..."
sleep 1

# Claim assets on L1
echo "Claiming assets on L1..."
L1_BALANCE_BEFORE_CLAIM=$(cast balance "$ACCOUNT" --rpc-url "$L1RPC")

L1_BALANCE_AFTER_CLAIM=$(cast balance "$ACCOUNT" --rpc-url "$L1RPC")
echo "Balance on L1:"
echo "  Before claiming: $L1_BALANCE_BEFORE_CLAIM"
echo "  After claiming: $L1_BALANCE_AFTER_CLAIM"