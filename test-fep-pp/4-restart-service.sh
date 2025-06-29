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

PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ ! -d "./xlayer-erigon" ]; then
  echo "Cloning contract repository..."
  git clone -b yz/pp_finalized_block https://github.com/okx/xlayer-erigon.git
fi

cd ./xlayer-erigon
echo "Cleaning and resting contract repository..."
rm -rf *; git reset --hard; git pull;  git checkout yz/pp_finalized_block
make build-docker

cd $PWD_DIR

DOCK_CONFIG_FILE="./docker-compose.yml"

DOCK_CONFIG_FILE="./config/test.erigon.seq.config.yaml"
sed_inplace "s|zkevm.executor-strict: false| |g" "$DOCK_CONFIG_FILE"
sed_inplace "s|zkevm.witness-full: false| |g" "$DOCK_CONFIG_FILE"
sed_inplace "s|zkevm.executor-mock: true| |g" "$DOCK_CONFIG_FILE"

DOCK_CONFIG_FILE="./config/test.erigon.rpc.config.yaml"
sed_inplace "s|zkevm.mock-witness-generation: true| |g" "$DOCK_CONFIG_FILE"

./5-build-ckd-node.sh 0
make run-new