#!/bin/bash

# Strict mode: exit on command failure or undefined variable
set -eu
# set -x

git checkout config/test.erigon.seq.config.yaml
git checkout config/test.erigon.rpc.config.yaml

./init.sh

sleep 30

# Test nomarl bridge
./10-bridge-eth.sh
./11-bridge-okb.sh

# Try to hard fork
./1-hard-fork.sh

# Claim OKB
sleep 120
./13-claim.sh

# Test nomarl bridge
./10-bridge-eth.sh
./11-bridge-okb.sh