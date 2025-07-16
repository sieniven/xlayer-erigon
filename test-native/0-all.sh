#!/bin/bash

# Strict mode: exit on command failure or undefined variable
set -eu
# set -x

git checkout config/test.erigon.seq.config.yaml
git checkout config/test.erigon.rpc.config.yaml

./init.sh

sleep 120

./11-bridge-okb.sh
./10-bridge-eth.sh
./2-deploy-mint.sh
./3-restart-seq.sh