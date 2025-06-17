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
docker stop xlayer-cdk-node && docker rm xlayer-cdk-node
docker stop xlayer-agglayer && docker rm xlayer-agglayer

DOCK_CONFIG_FILE="./config/test.erigon.seq.config.yaml"
#sed_inplace "s|zkevm.executor-strict: false| |g" "$DOCK_CONFIG_FILE"
#sed_inplace "s|zkevm.witness-full: false| |g" "$DOCK_CONFIG_FILE"
#sed_inplace "s|zkevm.executor-mock: true| |g" "$DOCK_CONFIG_FILE"
echo -e "\nzkevm.l2-perform-map: true" >> "$DOCK_CONFIG_FILE"

docker compose -f docker-compose.yml up -d xlayer-seq
sleep 30

docker stop xlayer-seq && docker rm xlayer-seq

rm -rf data/rpc/chaindata && rm -rf data/rpc/smt && rm -rf data/rpc/caplin && rm -rf data/rpc/snapshots
cp -rf data/seq/chaindata data/rpc/chaindata
cp -rf data/seq/smt data/rpc/smt
cp -rf data/seq/caplin data/rpc/caplin
cp -rf data/seq/snapshots data/rpc/snapshots

DOCK_CONFIG_FILE="./config/test.erigon.seq.config.yaml"
sed_inplace "s|zkevm.l2-perform-map: true| zkevm.l2-perform-map: false |g" "$DOCK_CONFIG_FILE"

docker compose -f docker-compose.yml up -d xlayer-agglayer
docker compose -f docker-compose.yml up -d xlayer-cdk-node
docker compose -f docker-compose.yml up -d xlayer-rpc
docker compose -f docker-compose.yml up -d xlayer-seq
