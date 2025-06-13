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


cd ./xlayer-contracts

BYTECODE=$(jq -r '.bytecode' "$CONTRACT_JSON")
sp1_contract_address=$(cast send --private-key $DEPLOYER_PRIVATE_KEY --create  "$BYTECODE" | awk '/contractAddress/ {print $2}')
echo "sp1_contract_address: $sp1_contract_address"

hex=$(cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeCount()")
rollupTypeCount=$((16#${hex#0x}))
echo "rollupTypeCount $rollupTypeCount"

currentRollupType=$(cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeMap(uint32)(address,address,uint64,uint8,bool,bytes32)" 0)
echo "currentRollupType $currentRollupType"

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
