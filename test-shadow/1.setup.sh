set -e 

source .env

CUR_DIR=$(pwd)

# 检查网络是否存在，不存在才创建
if ! docker network ls | grep -q "rpcs"; then
    docker network create rpcs
    echo "Created docker network: rpcs"
else
    echo "Docker network rpcs already exists"
fi

TMP_DIR=$CUR_DIR/tmp
if [ ! -d "$TMP_DIR" ]; then
    echo "Creating working directory structure..."
    mkdir  -p "$TMP_DIR/anvil"
    mkdir  -p "$TMP_DIR/agglayer"
    mkdir  -p "$TMP_DIR/aggkit"
    chmod -R 777 "$TMP_DIR"
fi
# cd "$TMP_DIR"
# if [ ! -d "aggkit-code" ]; then
#     git clone git@github.com:okx/aggkit.git aggkit-code
#     cd aggkit-code
#     git checkout feature/0.1.0
#     make build-docker
# else
#     cd aggkit-code
#     git checkout feature/0.1.0
#     git pull
#     make build-docker
# fi
# cd "$TMP_DIR"

# if [ ! -d "agglayer-contracts" ]; then
#     git clone git@github.com:agglayer/agglayer-contracts.git
#     cd agglayer-contracts
#     git checkout v11.0.0-rc.0
# else 
#     cd agglayer-contracts
#     rm -rf *; git reset --hard; 
#     git pull;  
#     git checkout v11.0.0-rc.0
# fi
# cd "$TMP_DIR"

# docker run -d -p 3000:8545 \
#     --rm --name anvil \
#     --network rpcs \
#     --entrypoint "anvil" \
#     ghcr.io/foundry-rs/foundry:latest \
#     --block-time 12 \
#     --host 0.0.0.0 \
#     --fork-url https://rpc.ankr.com/eth/${L1_KEY} \
#     --state $TMP_DIR/anvil \
#     --fork-block-number 22688021


# cast rpc --rpc-url http://127.0.0.1:3000 evm_setNextBlockTimestamp $(date +%s)
# cast rpc --rpc-url http://127.0.0.1:3000 anvil_setStorageAt 0xEf1462451C30Ea7aD8555386226059Fe837CA4EF $(cast to-uint256 2) $(cast to-uint256 1)

# cast wallet import --private-key 0x9cef1f40624aba3fa6a24c587dde060ab9aa823fef108db63fd0ba5f0a4ba830 --keystore-dir config/ agglayer.keystore
# cast wallet import --private-key 0x452e72182077e2bc90ad9a53afc1dc4476fa429cec9fc6a437fb95b791045d43 --keystore-dir config/ sequencer.keystore

cd $CUR_DIR

# docker run -d --rm \
#     --name agglayer-prover \
#     --network rpcs \
#     -v "$PWD/conf:/etc/agglayer:ro" \
#     -e "SP1_PRIVATE_KEY=${SP1_KEY}" \
#     -e "NETWORK_RPC_URL=https://rpc.production.succinct.xyz" \
#     -e "RUST_BACKTRACE=1" \
#     -e "NETWORK_PRIVATE_KEY=$SP1_KEY" \
#     --entrypoint agglayer \
#     ghcr.io/agglayer/agglayer:0.3.3 \
#     prover --cfg /etc/agglayer/agglayer-prover-config.toml


docker run -d --rm \
    --name agglayer-node \
    --network rpcs \
    -v "$CUR_DIR/conf:/etc/agglayer:ro" \
    -v "$TMP_DIR/agglayer:/var/agglayer" \
    --entrypoint agglayer \
    ghcr.io/agglayer/agglayer:0.3.3 \
    run --cfg /etc/agglayer/agglayer-config.toml