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

docker stop xlayer-seq && docker rm xlayer-seq
docker stop xlayer-rpc && docker rm xlayer-rpc

DOCK_CONFIG_FILE="./config/test.erigon.seq.config.yaml"
#sed_inplace "s|zkevm.executor-strict: false| |g" "$DOCK_CONFIG_FILE"
#sed_inplace "s|zkevm.witness-full: false| |g" "$DOCK_CONFIG_FILE"
#sed_inplace "s|zkevm.executor-mock: true| |g" "$DOCK_CONFIG_FILE"
echo -e "\nzkevm.l2-perform-map: true" >> "$DOCK_CONFIG_FILE"

docker compose -f docker-compose.yml up -d xlayer-seq
sleep 30

docker stop xlayer-seq && docker rm xlayer-seq

rm -rf data/rpc/chaindata && rm -rf data/rpc/smt
cp -rf data/seq/chaindata data/rpc/chaindata
cp -rf data/seq/smt data/rpc/smt

DOCK_CONFIG_FILE="./config/test.erigon.rpc.config.yaml"
#sed_inplace "s|zkevm.mock-witness-generation: true| |g" "$DOCK_CONFIG_FILE"

docker compose -f docker-compose.yml up -d xlayer-seq
docker compose -f docker-compose.yml up -d xlayer-rpc