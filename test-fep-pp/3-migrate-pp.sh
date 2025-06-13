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

hex=$(cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeCount()")
rollupTypeCount=$((16#${hex#0x}))
echo "$rollupTypeCount"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeMap(uint32)(address,address,uint64,uint8,bool,bytes32)" $rollupTypeCount

echo "Creating ./tools/initMigrationToPP/initMigrationToPP.json..."
cat > ./tools/initMigrationToPP/initMigrationToPP.json << EOF
{
    "type": "EOA",
    "polygonRollupManagerAddress": "0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a",
    "rollupID": 1,
    "newRollupTypeID": $rollupTypeCount,
    "timelockDelay": 0,
    "maxFeePerGas": "",
    "maxPriorityFeePerGas": "",
    "multiplierGas": ""
}
EOF

echo "Before initMigrationToPP.ts"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupIDToRollupData(uint32)(address,uint64,address,uint64,bytes32,uint64,uint64,uint64,uint64,uint64,uint64,uint8)" 1

npx hardhat run ./tools/initMigrationToPP/initMigrationToPP.ts --network localhost

echo "After initMigrationToPP.ts, rollupTypeID: 1"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupIDToRollupData(uint32)(address,uint64,address,uint64,bytes32,uint64,uint64,uint64,uint64,uint64,uint64,uint8)" 1