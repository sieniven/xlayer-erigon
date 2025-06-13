#!/bin/bash
set -e
# set -x

DEPLOYER_ADDRESS="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
DEPLOYER_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
TIME_LOCK_ADDRESS="0xEA8DCb15a6AC928C1Bf07bD677682d59d48d9eC8"
ROLLUP_MGR_ADDRESS="0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a"
L1_RPC_URL="http://127.0.0.1:8545"


PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$PWD_DIR")"

docker stop xlayer-seqs; docker rm xlayer-seqs
WAIT_DELAY=1  # seconds

# Function to make JSON-RPC calls
call_rpc() {
    local method=$1
    local params=$2
    local id=$3
    curl -s --location "$L2_RPC_URL" \
        --header 'Content-Type: application/json' \
        --data "{
            \"id\": $id,
            \"jsonrpc\": \"2.0\",
            \"method\": \"$method\",
            \"params\": $params
        }"
}

# Function to convert hex to decimal
hex_to_dec() {
    local hex=$1
    echo $((hex))
}

# Wait for verified batch to match virtual batch
echo "Checking batch synchronization..."
while true; do
    virtual=$(call_rpc "zkevm_virtualBatchNumber" "[]" 1 | jq -r '.result')
    verified=$(call_rpc "zkevm_verifiedBatchNumber" "[]" 2 | jq -r '.result')

    virtual_dec=$(hex_to_dec "$virtual")
    verified_dec=$(hex_to_dec "$verified")

    echo "Virtual batch: $virtual_dec, Verified batch: $verified_dec"

    if [ "$virtual_dec" -eq "$verified_dec" ]; then
        echo "Verified batch synchronized"
        break
    fi

    sleep $WAIT_DELAY
done

# Find latest consolidated block
echo "Finding latest consolidated block..."

current_block_hex=$(call_rpc "eth_blockNumber" "[]" 83 | jq -r '.result')
current_block=$(hex_to_dec "$current_block_hex")
echo "Current block height: $current_block"

while [ "$current_block" -ge 0 ]; do
    consolidated=$(call_rpc "zkevm_isBlockConsolidated" "[$current_block]" 1 | jq -r '.result')

    if [ "$consolidated" = "true" ]; then
        echo "Block $current_block is consolidated"
        if [ "$FORCE_STOP_BLOCK" == "true" ]; then
          CDK_CONFIG_FILE="config/cdk-node-config.toml"
          sed_inplace "s|ForcedStopBlock = 0|ForcedStopBlock = $current_block|g" "$CDK_CONFIG_FILE"
        fi
        break
    else
        echo "Block $current_block not consolidated. Checking previous block..."
        current_block=$((current_block - 1))
    fi
done

make stop-old

cd ./xlayer-contracts

echo "Creating ./upgrade/upgrade-rollupManager-v0.3.1/upgrade_parameters.json..."
cat > ./upgrade/upgrade-rollupManager-v0.3.1/upgrade_parameters.json << EOF
{

   "tagSCPreviousVersion": "v1.0.0",
    "tagSCNewVersion": "v1.0.0",
    "rollupManagerAddress": "0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a",
    "test": false,
    "timelockDelay": 60
}
EOF


npx hardhat run ./upgrade/upgrade-rollupManager-v0.3.1/upgrade-rollupManager-v0.3.1.ts --network localhost
