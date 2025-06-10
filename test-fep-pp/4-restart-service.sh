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
sed_inplace "s|image: cdk-erigon|zjg555543/xlayer-erigon:dev-pp-6dd537c|g" "$DOCK_CONFIG_FILE"



make run-new