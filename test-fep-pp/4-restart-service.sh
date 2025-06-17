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

DOCK_CONFIG_FILE="./docker-compose.yml"
#sed_inplace "s|image: cdk-erigon|image: zjg555543/xlayer-erigon:dev-pp-6dd537c|g" "$DOCK_CONFIG_FILE"

DOCK_CONFIG_FILE="./config/test.erigon.seq.config.yaml"
#sed_inplace "s|zkevm.executor-strict: false| |g" "$DOCK_CONFIG_FILE"
#sed_inplace "s|zkevm.witness-full: false| |g" "$DOCK_CONFIG_FILE"
#sed_inplace "s|zkevm.executor-mock: true| |g" "$DOCK_CONFIG_FILE"

DOCK_CONFIG_FILE="./config/test.erigon.rpc.config.yaml"
sed_inplace "s|zkevm.mock-witness-generation: true| |g" "$DOCK_CONFIG_FILE"

./5-build-ckd-node.sh 0
make run-new