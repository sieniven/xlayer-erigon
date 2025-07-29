#!/bin/bash

# This should be run in the root directory of the repo
ROOT_DIR=$(git rev-parse --show-toplevel)
TEST_DIR="$ROOT_DIR/test-pp-op"
TMP_DIR="$TEST_DIR/tmp"
SA_BENCH_DIR="$TMP_DIR/SA-Benchmark"

# Clone relevant repos
function clone_repos {
    cd $TMP_DIR
    if [ ! -d $SA_BENCH_DIR ]; then
        git clone -b dumi/senddet git@github.com:okx/SA-Benchmark.git
    fi
    cd $ROOT_DIR
}

clone_repos

# 1. Run SA-Benchmark setup only for state0
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
sleep 5
cd $TEST_DIR
docker compose stop xlayer-seq
cp -r data data_state0

# 2. Send one tx and save state1.json
cd $TEST_DIR
docker compose start xlayer-seq
sleep 5
cast send 0xa03666Fb51Aa9aD2DE70e0434072A007b3C91A9E --value 200000 \
--private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 \
--legacy --gas-price 100000000 \
--rpc-url http://localhost:8123
sleep 5
cd $TEST_DIR
docker compose stop xlayer-seq
cp -r data data_state1

# 3. Send deterministic tx and save state2.json
cd $TEST_DIR
docker compose start xlayer-seq
sleep 5
cd $SA_BENCH_DIR
git checkout dumi/senddet
yarn run senduop:local
sleep 5
cd $TEST_DIR
docker compose stop xlayer-seq
cp -r data data_state2