#!/bin/bash

# This should be run in the root directory of the repo
ROOT_DIR=$(git rev-parse --show-toplevel)
TEST_DIR="$ROOT_DIR/test-pp-op"
TMP_DIR="$TEST_DIR/tmp"
SA_BENCH_DIR="$TMP_DIR/SA-Benchmark"

RPC_URL="http://localhost:8123"

TIME_STAMP=$(date +%Y%m%d-%H%M%S)
RESULT_FILE="check-regenesis-result-$TIME_STAMP.txt"

# 1. Run state-check state0
cd $ROOT_DIR
go install ./cmd/state-check/
cd $TEST_DIR
state-check -dump-state-file config-op/state0.json -rpc-url $RPC_URL

# 8. Run state-check state1
cast send 0xa03666Fb51Aa9aD2DE70e0434072A007b3C91A9E --value 200000 \
--private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 \
--legacy --gas-price 10000000000 \
--rpc-url $RPC_URL
sleep 5
state-check -dump-state-file config-op/state1.json -rpc-url $RPC_URL

# 9. Run state-check state2
cd $SA_BENCH_DIR
yarn run senduop:deterministicop
sleep 5
cd $TEST_DIR
state-check -dump-state-file config-op/state2.json -rpc-url $RPC_URL