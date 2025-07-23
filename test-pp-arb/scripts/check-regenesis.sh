#!/bin/bash

TMP_DIR="temp"
rm -rf $TMP_DIR
mkdir -p $TMP_DIR
cd $TMP_DIR
ROOT_DIR=$(pwd)

ARB_DIR="nitro-testnode"
XLAYER_ERIGON_DIR="xlayer-erigon"
SA_BENCH_DIR="SA-Benchmark"

TIME_STAMP=$(date +%Y%m%d-%H%M%S)
RESULT_FILE="check-regenesis-result-$TIME_STAMP.txt"

# Clone relevant repos
function clone_repos {
    git clone -b dumi --recurse-submodules https://github.com/liudi4046/nitro-testnode.git $ARB_DIR
    git clone -b dumi/regenesis-pp https://github.com/okx/xlayer-erigon.git $XLAYER_ERIGON_DIR
    git clone -b dumi/senddet git@github.com:okx/SA-Benchmark.git $SA_BENCH_DIR
}

clone_repos

# Stop all services
function stop_all {
    cd $ROOT_DIR
    cd $XLAYER_ERIGON_DIR/test && make stop
    cd $ROOT_DIR
    cd $ARB_DIR && ./test-node.bash --stop
}

stop_all

# 1. Start xlayer-erigon in testnet mode on dev-pp branch
cd $ROOT_DIR
cd $XLAYER_ERIGON_DIR/test && make min-run
cd $ROOT_DIR
sleep 5

# 2. Run SA-Benchmark setup only
cd $SA_BENCH_DIR
cp example.env .env
export NVM_DIR="$HOME/.nvm"
if ! [ -s "$NVM_DIR/nvm.sh" ]; then
    echo "nvm not found, installing..."
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash
fi
. "$NVM_DIR/nvm.sh"
nvm use v22
./1-setup.sh
cd $ROOT_DIR

# 3. Stop xlayer-erigon and generate genesis
cd $XLAYER_ERIGON_DIR/test && mv data saved_data && make stop && mv saved_data data
cd ..
./test-pp-arb/scripts/gen-arb-testnet-genesis.sh
cp genesis.json ../$ARB_DIR/genesis.json
mv genesis.json state0.json
cd $ROOT_DIR

# 4. Re-start xlayer-erigon and send one tx
cd $XLAYER_ERIGON_DIR/test && make min-run
sleep 5
cast send 0xa03666Fb51Aa9aD2DE70e0434072A007b3C91A9E --value 200000 \
--private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 \
--legacy --gas-price 100000000 \
--rpc-url http://localhost:8123
sleep 5
mv data saved_data && make stop && mv saved_data data
cd ..
./test-pp-arb/scripts/gen-arb-testnet-genesis.sh
mv genesis.json state1.json
cd $ROOT_DIR

# 5. Re-start xlayer-erigon and send deterministic tx
cd $XLAYER_ERIGON_DIR/test && make min-run
cd $ROOT_DIR
sleep 5
cd $SA_BENCH_DIR
git checkout dumi/senddet
yarn run senduop:local
sleep 5
cd $ROOT_DIR
cd $XLAYER_ERIGON_DIR/test && mv data saved_data && make stop && mv saved_data data
cd ..
./test-pp-arb/scripts/gen-arb-testnet-genesis.sh
mv genesis.json state2.json
cd $ROOT_DIR

# 6. Start nitro-testnode
cd $ARB_DIR && ./test-node.bash --init-force --detach
sleep 5
cd $ROOT_DIR

# 7. Run state-check state0
cd $XLAYER_ERIGON_DIR
make state-check
./build/bin/state-check -dump-state-file state0.json -rpc-url http://localhost:8547 -progress-bar=false> $RESULT_FILE 2>&1

# 8. Run state-check state1
cast send 0xa03666Fb51Aa9aD2DE70e0434072A007b3C91A9E --value 200000 \
--private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 \
--legacy --gas-price 100000000 \
--rpc-url http://localhost:8547
sleep 3
./build/bin/state-check -dump-state-file state1.json -rpc-url http://localhost:8547 -progress-bar=false >> $RESULT_FILE 2>&1
cd $ROOT_DIR

# 9. Run state-check state2
cd $SA_BENCH_DIR
yarn run senduop:deterministic
sleep 3
cd $ROOT_DIR
cd $XLAYER_ERIGON_DIR
make state-check
./build/bin/state-check -dump-state-file state2.json -rpc-url http://localhost:8547 -progress-bar=false >> $RESULT_FILE 2>&1
mv $RESULT_FILE ../../
cd $ROOT_DIR

# stop_all

# 8. Print result
# cd $ROOT_DIR
# echo -e "\n\n\n"
# cat ../$RESULT_FILE
# echo -e "\n\n\n"