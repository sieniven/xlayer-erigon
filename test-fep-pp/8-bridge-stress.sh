#!/bin/bash

# Strict mode: exit on command failure or undefined variable
set -eu
# set -x

input=${1:-0}

# checking input
if [ "$input" == 0 ]; then
    echo "Error input value, must larger than 0" >&2
    exit 1
fi

# =============================================================================
# Configuration
# =============================================================================
BRIDGE_ADDRESS="0x3a277Fa4E78cc1266F32E26c467F99A8eAEfF7c3"
ACCOUNT="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534" 
PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
BRIDGE_VALUE_BIG="10"  # 1 ETH in wei
BRIDGE_VALUE_SMALL="1"  # 0.1 ETH in wei

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

count=0
CURRENT_NONCE=$(cast nonce --rpc-url $L1RPC $ACCOUNT)
echo "L1-L2, Current nonce for $ACCOUNT on L1: $CURRENT_NONCE"
while [ $count -lt $input ]
do
    cast send \
        --legacy \
        --rpc-url $L1RPC \
        --async \
        --gas-price 1gwei \
        --gas-limit 1000000 \
        --nonce $CURRENT_NONCE \
        --private-key $PRIVATE_KEY \
        --value $BRIDGE_VALUE_BIG \
        $BRIDGE_ADDRESS \
        'function bridgeAsset(uint32 destinationNetwork, address destinationAddress, uint256 amount, address token, bool forceUpdateGlobalExitRoot, bytes permitData) returns()' \
        1 $ACCOUNT $BRIDGE_VALUE_BIG $L1_ETH_ADDRESS true 0x
    count=$((count + 1))
    if (( count % 100 == 0 )); then
        echo "Waiting for 100 transactions to be processed,  $count of $input"
    fi
    sleep 0.002
    CURRENT_NONCE=$((CURRENT_NONCE + 1))
done

# Wait for assets to be claimed automatically by sponsor
echo "Waiting 120s for assets to be claimed by sponsor..."
sleep 120
while true; do
    balance=$(cast call "$L2_WETH" "balanceOf(address)(uint256)" "$ACCOUNT" --rpc-url "$L2RPC" | awk '{print $1}')
    balance=${balance:-0}
    increment=$(echo "$balance - $L2_BALANCE_BEFORE_BRIDGE" | bc)
    echo "Current balance on L2: $balance (increment: $increment)"
    if [ "$increment" -gt 0 ]; then
        echo "Assets successfully claimed on L2"
        break
    fi
    
    sleep 10
done
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
count=0
CURRENT_NONCE=$(cast nonce --rpc-url $L2RPC $ACCOUNT)
echo "L2-L1, Current nonce for $ACCOUNT on L2: $CURRENT_NONCE"

TX_HASH_ARRAY=()
while [ $count -lt $input ]
do
    TX_HASH=$(cast send \
        --legacy \
        --private-key $PRIVATE_KEY \
        --rpc-url $L2RPC \
        --gas-price 1gwei \
        --gas-limit 1000000 \
        --async \
        --nonce $CURRENT_NONCE \
        $BRIDGE_ADDRESS \
        'function bridgeAsset(uint32 destinationNetwork, address destinationAddress, uint256 amount, address token, bool forceUpdateGlobalExitRoot, bytes permitData) returns()' \
        0 $ACCOUNT $BRIDGE_VALUE_SMALL $L2_WETH true "0x")
    TX_HASH_ARRAY+=("$TX_HASH")

    count=$((count + 1))
    if (( count % 100 == 0 )); then
        echo "Waiting for 100 transactions to be processed,  $count of $input"
    fi
    sleep 0.002
    CURRENT_NONCE=$((CURRENT_NONCE + 1))
    echo "Bridge L2 tx TX_HASH: $TX_HASH"
done

# Wait for GER update on L1
echo "Waiting for GER to be updated on L1..."
while true; do
    GER_NEW=$(cast call "$GER_MGR" "getLastGlobalExitRoot()" --rpc-url "$L1RPC")
    if [ "$GER_NEW" != "$GER" ]; then
        GER=$GER_NEW
        break
    fi
    echo "Current GER on L1: $GER, waiting for GER to be updated on L1..."
    sleep 10
done

sleep_time=$((input / 5))

if [ $sleep_time -lt 180 ]; then
    sleep_time=180
fi

echo "GER updated to $GER on L1, and sleep $sleep_time seconds for all txs"
sleep $sleep_time

CURRENT_NONCE=$(cast nonce --rpc-url $L1RPC $ACCOUNT)
count=0
echo "L1 claim, Current nonce for $ACCOUNT on L1: $CURRENT_NONCE"
result=$(curl -s "$BRIDGE_SERVICE1/bridges/$ACCOUNT?limit=18000&offset=0" | \
   jq -r '.deposits[] | select(.ready_for_claim == true and .claim_tx_hash == "" and .tx_hash=="'$TX_HASH'")') 
for TX_HASH in "${TX_HASH_ARRAY[@]}"; do                                                                 
    DEPOSIT_CNT=$(echo "$result" | jq -r '.deposit_cnt')
    NETWORK_ID=$(echo "$result" | jq -r '.network_id')
    GLOBAL_INDEX=$(echo "$result" | jq -r '.global_index')
    ORINGIN_NETWORK=$(echo "$result" | jq -r '.orig_net')
    ORINGIN_ADDRESS=$(echo "$result" | jq -r '.orig_addr')
    DESTINATION_NETWORK=$(echo "$result" | jq -r '.dest_net')
    IN_AMOUNT=$(echo "$result" | jq -r '.amount')
    METADATA=$(echo "$result" | jq -r '.metadata')

    proof=$(curl -s "$BRIDGE_SERVICE1/merkle-proof?deposit_cnt=$DEPOSIT_CNT&net_id=$NETWORK_ID" | jq -r '.')
    MERKLE_PROOF=$(echo "$proof" | jq -r -c '.proof | .merkle_proof' | tr -d '"')
    ROLLUP_MERKLE_PROOF=$(echo "$proof" | jq -r -c '.proof | .rollup_merkle_proof' | tr -d '"')
    MER=$(echo "$proof" | jq -r '.proof | .main_exit_root')
    RER=$(echo "$proof" | jq -r '.proof | .rollup_exit_root')
    if [ "$MERKLE_PROOF" == "null" ] || [ "$MERKLE_PROOF" == "[]" ]; then
        echo "Error: Merkle proof is null, skipping claim for this transaction, and waiting for 60s, $TX_HASH" >&2
        sleep 0.001
        continue
    fi
    count=$((count + 1))
    # Claim assets on L1
    TX_HASH=$(cast send --legacy --rpc-url $L1RPC  --gas-price 1gwei --gas-limit 1000000 --async --nonce $CURRENT_NONCE --private-key $PRIVATE_KEY $BRIDGE_ADDRESS 'claimAsset(bytes32[32],bytes32[32],uint256,bytes32,bytes32,uint32,address,uint32,address,uint256,bytes)' $MERKLE_PROOF $ROLLUP_MERKLE_PROOF $GLOBAL_INDEX $MER $RER $ORINGIN_NETWORK $ORINGIN_ADDRESS $DESTINATION_NETWORK $ACCOUNT $IN_AMOUNT $METADATA)
    echo "Claim transaction hash: $TX_HASH"
    sleep 0.002
    CURRENT_NONCE=$((CURRENT_NONCE + 1))
    if (( count % 100 == 0 )); then
        echo "Waiting for 100 transactions to be processed,  $count of $input"
    fi
done

echo "L1 claim, and sleep $sleep_time seconds for all txs"
sleep $sleep_time
