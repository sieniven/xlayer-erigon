#!/bin/bash
set -e
# set -x

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

if [ -f .env ]; then
    source .env
    if [ "$PROVER_TYPE" != "mock" ] && [ "$PROVER_TYPE" != "true" ]; then
      echo "Error: Invalid ProverType '$1'. Only 'mock' or 'true' are allowed."
      exit 1
    fi
else
    echo "Error: .env file not found"
    exit 1
fi

echo "Using ProverType: $PROVER_TYPE"

CONFIG_FILE_1="./config/agglayer-config.toml"
CONFIG_FILE_2="./config/agglayer-prover-config.toml"
CONTRACT_JSON="./artifacts/contracts/mocks/VerifierRollupHelperMock.sol/VerifierRollupHelperMock.json"
if [ "$PROVER_TYPE" == "true" ]; then
    CONTRACT_JSON="./artifacts/contracts/verifiers/v4.0.0-rc.3/SP1VerifierPlonk.sol/SP1VerifierPlonk.json"
    sed_inplace "s|mock-verifier *= *true|mock-verifier = false|g" "$CONFIG_FILE_1"
    sed_inplace "s|\[primary-prover\.mock-prover\]|\[primary-prover.cpu-prover\]|g" "$CONFIG_FILE_2"
else
    sed_inplace "s|mock-verifier *= *false|mock-verifier = true|g" "$CONFIG_FILE_1"
    sed_inplace "s|\[primary-prover\.cpu-prover\]|\[primary-prover.mock-prover\]|g" "$CONFIG_FILE_2"
fi

DEPLOYER_ADDRESS="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
DEPLOYER_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
TIME_LOCK_ADDRESS="0xEA8DCb15a6AC928C1Bf07bD677682d59d48d9eC8"
ROLLUP_MGR_ADDRESS="0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a"
L1_RPC_URL="http://127.0.0.1:8545"
L2_RPC_URL="http://127.0.0.1:8124"

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
        CDK_CONFIG_FILE="config/cdk-node-config.toml"
        sed_inplace "s|ForceStopBlock = 0|ForceStopBlock = $current_block|g" "$CDK_CONFIG_FILE"
        break
    else
        echo "Block $current_block not consolidated. Checking previous block..."
        current_block=$((current_block - 1))
    fi
done

make stop-old

cd ./xlayer-contracts

BYTECODE=$(jq -r '.bytecode' "$CONTRACT_JSON")
sp1_contract_address=$(cast send --private-key $DEPLOYER_PRIVATE_KEY --create  "$BYTECODE" | awk '/contractAddress/ {print $2}')
echo "sp1_contract_address: $sp1_contract_address"

cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeCount()" 
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeMap(uint32)(address,address,uint64,uint8,bool,bytes32)" 1 

echo "Creating ./tools/addRollupType/add_rollup_type.json..."
cat > ./tools/addRollupType/add_rollup_type.json << EOF
{
    "type": "EOA",
    "consensusContract": "PolygonPessimisticConsensus",
    "polygonRollupManagerAddress": "0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a",
    "polygonZkEVMBridgeAddress": "0x3a277Fa4E78cc1266F32E26c467F99A8eAEfF7c3",
    "polygonZkEVMGlobalExitRootAddress": "0xB8cedD4B9eF683f0887C44a6E4312dC7A6e2fcdB",
    "polTokenAddress": "0x5FbDB2315678afecb367f032d93F642f64180aa3",
    "verifierAddress": "$sp1_contract_address",
    "description": "Fork13 PP",
    "forkID": 13,
    "rollupCompatibilityID": 0,
    "timelockDelay": 60,
    "timelockSalt": "",
    "deployerPvtKey": "",
    "maxFeePerGas":"",
    "maxPriorityFeePerGas":"",
    "multiplierGas": "",
    "genesisRoot": "0xb014eb12e554f165a523f44272e2f5952cb2e1fe8dac595b0e9db2dbe39e9f21",
    "programVKey": "0x00d6e4bdab9cac75a50d58262bb4e60b3107a6b61131ccdff649576c624b6fb7"
}
EOF

cp ../contract/genesis.json ./tools/addRollupType/genesis.json

npx hardhat run ./tools/addRollupType/addRollupType.ts --network localhost

hex=$(cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeCount()")
rollupTypeCount=$((16#${hex#0x}))
echo "$rollupTypeCount"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeMap(uint32)(address,address,uint64,uint8,bool,bytes32)" $rollupTypeCount

echo "Creating ./tools/updateRollup/updateRollup.json..."
cat > ./tools/updateRollup/updateRollup.json << EOF
{
    "type": "EOA",
    "polygonRollupManagerAddress": "0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a",
    "timelockDelay": 0,
    "deployerPvtKey": "",
    "maxFeePerGas": "",
    "maxPriorityFeePerGas": "",
    "multiplierGas": "",
    "rollups": [
        {
            "rollupAddress": "0xeb173087729c88a47568AF87b17C653039377BA6",
            "newRollupTypeID": $rollupTypeCount,
            "upgradeData": "0x"
        }
    ]
}
EOF

echo "Before updateRollup.ts"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupIDToRollupData(uint32)(address,uint64,address,uint64,bytes32,uint64,uint64,uint64,uint64,uint64,uint64,uint8)" 1 

npx hardhat run ./tools/updateRollup/updateRollup.ts --network localhost

echo "After updateRollup.ts, rollupTypeID: 1"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupIDToRollupData(uint32)(address,uint64,address,uint64,bytes32,uint64,uint64,uint64,uint64,uint64,uint64,uint8)" 1 
