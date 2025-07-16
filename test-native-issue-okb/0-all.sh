#!/bin/bash

# Strict mode: exit on command failure or undefined variable
set -eu
# set -x

git checkout config/test.erigon.seq.config.yaml
git checkout config/test.erigon.rpc.config.yaml

make stop
make run

sleep 10

./10-bridge-eth.sh
./11-bridge-okb.sh
./2-deploy-mint.sh
./3-restart-seq.sh