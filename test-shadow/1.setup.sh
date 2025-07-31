set -e 

source .env

if [ -z "$L1_URL" ] || [ -z "$SP1_KEY" ] ; then
    echo "L1_URL, SP1_KEY must be set"
    exit 1
fi

if [ $(docker ps -aq | wc -l) -gt 0 ]; then
    docker stop $(docker ps -aq)
    docker rm $(docker ps -aq)
fi

CUR_DIR=$(pwd)

if ! docker network ls | grep -q "rpcs"; then
    docker network create rpcs
    echo "Created docker network: rpcs"
else
    echo "Docker network rpcs already exists"
fi

TMP_DIR=$CUR_DIR/tmp
CODE_DIR=$CUR_DIR/code

rm -rf $TMP_DIR
if [ ! -d "$TMP_DIR" ]; then
    echo "Creating working directory structure..."
    mkdir  -p "$TMP_DIR/anvil"
    mkdir  -p "$TMP_DIR/agglayer"
    mkdir  -p "$TMP_DIR/aggkit"
    chmod -R 777 "$TMP_DIR"
fi

cd "$CODE_DIR"
if [ ! -d "aggkit-code" ]; then
    git clone git@github.com:okx/aggkit.git aggkit-code
    cd aggkit-code
    git checkout feature/0.1.0
    make build-docker
else
    cd aggkit-code
    git checkout feature/0.1.0
    git pull
    make build-docker
fi
cd "$CODE_DIR"

if [ ! -d "agglayer-contracts" ]; then
    git clone git@github.com:agglayer/agglayer-contracts.git
    cd agglayer-contracts
    git checkout v11.0.0-rc.0
else 
    cd agglayer-contracts
    rm -rf *; 
    rm -rf .openzeppelin;
    git reset --hard; 
    git checkout v11.0.0-rc.0
fi
cd "$CUR_DIR"

docker run -d -p 3000:8545 \
    --name anvil \
    --network rpcs \
    -v $TMP_DIR/anvil:/anvil-state \
    --entrypoint "anvil" \
    ghcr.io/foundry-rs/foundry:latest \
    --block-time 12 \
    --host 0.0.0.0 \
    --fork-url ${L1_URL} \
    --state /anvil-state \
    --fork-block-number 22688021

echo "sleep 10 for anvil to start"
sleep 10

cast rpc --rpc-url http://127.0.0.1:3000 evm_setNextBlockTimestamp $(date +%s)
cast rpc --rpc-url http://127.0.0.1:3000 anvil_setStorageAt 0xEf1462451C30Ea7aD8555386226059Fe837CA4EF $(cast to-uint256 2) $(cast to-uint256 1)

cd $CUR_DIR

docker run -d \
    --name agglayer-prover \
    --network rpcs \
    -v "$PWD/conf:/etc/agglayer:ro" \
    -e "SP1_PRIVATE_KEY=${SP1_KEY}" \
    -e "NETWORK_RPC_URL=https://rpc.production.succinct.xyz" \
    -e "RUST_BACKTRACE=1" \
    -e "NETWORK_PRIVATE_KEY=$SP1_KEY" \
    --entrypoint agglayer \
    ghcr.io/agglayer/agglayer:0.3.3 \
    prover --cfg /etc/agglayer/agglayer-prover-config.toml

echo "sleep 10 for agglayer-prover to start"
sleep 10

docker run -d \
    --name agglayer-node \
    --network rpcs \
    -v "$CUR_DIR/conf:/etc/agglayer:ro" \
    -v "$TMP_DIR/agglayer:/var/agglayer" \
    --entrypoint agglayer \
    ghcr.io/agglayer/agglayer:0.3.3 \
    run --cfg /etc/agglayer/agglayer-config.toml

echo "sleep 10 for agglayer-node to start"
sleep 10

cast rpc --rpc-url http://127.0.0.1:3000 anvil_impersonateAccount 0xa90b4c8b8807569980f6cc958c8905383136b5ea

cast send --unlocked --from 0xa90b4c8b8807569980f6cc958c8905383136b5ea --rpc-url http://127.0.0.1:3000 0x2B0ee28D4D51bC9aDde5E58E295873F61F4a0507 'setTrustedSequencer(address)' 0x8Ad44b2b5368a3043901ee373dC6D400c6A2e83F

cast rpc --rpc-url http://127.0.0.1:3000 anvil_stopImpersonatingAccount 0xa90b4c8b8807569980f6cc958c8905383136b5ea


cast rpc --rpc-url http://127.0.0.1:3000 anvil_impersonateAccount 0x242daE44F5d8fb54B198D03a94dA45B5a4413e21

cast send --unlocked --from 0x242daE44F5d8fb54B198D03a94dA45B5a4413e21 --rpc-url http://127.0.0.1:3000 0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2 'grantRole(bytes32 role, address account)' $(cast keccak TRUSTED_AGGREGATOR_ROLE) 0xaff8Ed903d079cD0E7fE29138b37B6AC8fFe4AdF

cast rpc --rpc-url http://127.0.0.1:3000 anvil_stopImpersonatingAccount 0x242daE44F5d8fb54B198D03a94dA45B5a4413e21

cast rpc --rpc-url http://127.0.0.1:3000 anvil_setBalance 0xaff8Ed903d079cD0E7fE29138b37B6AC8fFe4AdF 1000000000000000000

cd $CODE_DIR/agglayer-contracts/upgrade/upgrade-rollupManager-v0.3.1/

jq '.tagSCPreviousVersion = "FEP-v10.0.0-rc.0"' upgrade_parameters.json.example > _t; mv _t upgrade_parameters.json
jq '.rollupManagerAddress = "0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2"' upgrade_parameters.json > _t; mv _t upgrade_parameters.json
jq '.timelockDelay = "60"' upgrade_parameters.json > _t; mv _t upgrade_parameters.json
jq '.timelockSalt = "0x0000000000000000000000000000000000000000000000000000000000000000"' upgrade_parameters.json > _t; mv _t upgrade_parameters.json
jq '.test = true' upgrade_parameters.json > _t; mv _t upgrade_parameters.json


cd $CODE_DIR/agglayer-contracts
mkdir $CODE_DIR/agglayer-contracts/.openzeppelin
cp upgrade/upgradePessimistic/mainnet-info/mainnet.json $CODE_DIR/agglayer-contracts/.openzeppelin/mainnet.json
git config --global --add safe.directory $CODE_DIR/agglayer-contracts
npm i

export MAINNET_PROVIDER=http://127.0.0.1:3000
npx hardhat run ./upgrade/upgrade-rollupManager-v0.3.1/upgrade-rollupManager-v0.3.1.ts --network mainnet


cat << EOF > tools/addRollupType/add_rollup_type.json
{
    "type": "Timelock",
    "consensusContract": "PolygonPessimisticConsensus",
    "consensusContractAddress": "0x18C45DD422f6587357a6d3b23307E75D42b2bc5B",
    "polygonRollupManagerAddress": "0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2",
    "verifierAddress": "0x0459d576A6223fEeA177Fb3DF53C9c77BF84C459",
    "description": "Type: Pessimistic, Version: v0.3.3, genesis: /ipfs/QmUXnRoPbUmZuEZCGyiHjEsoNcFVu3hLtSvhpnfBS2mAYU",
    "forkID": 12,
    "timelockDelay": 60,
    "programVKey": "0x00eff0b6998df46ec388bb305618089ae3dc74e513e7676b2e1909694f49cc30",
    "outputPath": "add_rollup_type_output.json"
}
EOF

npx hardhat run ./tools/addRollupType/addRollupType.ts --network mainnet

cd $CUR_DIR
cast rpc --rpc-url http://127.0.0.1:3000 anvil_impersonateAccount 0x242dae44f5d8fb54b198d03a94da45b5a4413e21


cast send \
    --unlocked \
    --from 0x242dae44f5d8fb54b198d03a94da45b5a4413e21 \
    --rpc-url http://127.0.0.1:3000 \
    $(jq -r '.timelockContractAddress' $CODE_DIR/agglayer-contracts/upgrade/upgrade-rollupManager-v0.3.1/upgrade_output.json) \
    $(jq -r '.scheduleData' $CODE_DIR/agglayer-contracts/tools/addRollupType/add_rollup_type_output.json)

echo "sleep 60 for timelock to schedule"
sleep 60

cast send \
    --unlocked \
    --from 0x242dae44f5d8fb54b198d03a94da45b5a4413e21 \
    --rpc-url http://127.0.0.1:3000 \
    $(jq -r '.timelockContractAddress' $CODE_DIR/agglayer-contracts/upgrade/upgrade-rollupManager-v0.3.1/upgrade_output.json) \
    $(jq -r '.executeData' $CODE_DIR/agglayer-contracts/tools/addRollupType/add_rollup_type_output.json)


cast send \
    --unlocked \
    --from 0x242dae44f5d8fb54b198d03a94da45b5a4413e21 \
    --rpc-url http://127.0.0.1:3000 \
    $(jq -r '.timelockContractAddress' $CODE_DIR/agglayer-contracts/upgrade/upgrade-rollupManager-v0.3.1/upgrade_output.json) \
    $(jq -r '.scheduleData' $CODE_DIR/agglayer-contracts/upgrade/upgrade-rollupManager-v0.3.1/upgrade_output.json)

echo "sleep 60 for timelock to execute"
sleep 60

cast send \
    --unlocked \
    --from 0x242dae44f5d8fb54b198d03a94da45b5a4413e21 \
    --rpc-url http://127.0.0.1:3000 \
    $(jq -r '.timelockContractAddress' $CODE_DIR/agglayer-contracts/upgrade/upgrade-rollupManager-v0.3.1/upgrade_output.json) \
    $(jq -r '.executeData' $CODE_DIR/agglayer-contracts/upgrade/upgrade-rollupManager-v0.3.1/upgrade_output.json)


cast rpc --rpc-url http://127.0.0.1:3000 anvil_stopImpersonatingAccount 0x242dae44f5d8fb54b198d03a94da45b5a4413e21


VERSION=$(cast call --rpc-url http://127.0.0.1:3000 0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2 "ROLLUP_MANAGER_VERSION()(string)")
echo "Current version: $VERSION"
EXPECTED_VERSION='"al-v0.3.1"'
if [ "$VERSION" != "$EXPECTED_VERSION" ]; then
    echo "❌ ERROR: Expected version $EXPECTED_VERSION, but got $VERSION"
    exit 1
fi

ROLLUP_TYPE_COUNT=$(cast call --rpc-url http://127.0.0.1:3000 0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2 "rollupTypeCount() external view returns (uint32)")
echo "Current rollup type count: $ROLLUP_TYPE_COUNT"
EXPECTED_COUNT="11"
if [ "$ROLLUP_TYPE_COUNT" != "$EXPECTED_COUNT" ]; then
    echo "❌ ERROR: Expected rollup type count $EXPECTED_COUNT, but got $ROLLUP_TYPE_COUNT"
    exit 1
fi

cast rpc --rpc-url http://127.0.0.1:3000 anvil_impersonateAccount 0x242dae44f5d8fb54b198d03a94da45b5a4413e21


cast send \
    --unlocked \
    --from 0x242dae44f5d8fb54b198d03a94da45b5a4413e21 \
    --rpc-url http://127.0.0.1:3000 \
    0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2 "initMigrationToPP(uint32,uint32)" 3 11

cast rpc --rpc-url http://127.0.0.1:3000 anvil_stopImpersonatingAccount 0x242dae44f5d8fb54b198d03a94da45b5a4413e21

docker run -d \
    --name aggkit \
    --network rpcs \
    -v $TMP_DIR/aggkit:/tmp \
    -v $CUR_DIR/conf:/etc/aggkit \
    aggkit:local run --cfg=/etc/aggkit/aggkit.toml --components=aggsender






