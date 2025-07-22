#!/bin/bash

if ! [ -f "cmd/hack/hack.go" ]; then
    echo "hack.go source code not found. Ensure you are in the root directory of the repository."
    exit 1
fi

rm -rf genesis.json
make hack
chaindata_dir=test/data/seq/chaindata
genesis_stub=test-pp-arb/config/arbitrum_testnet_genesis_stub.json
./build/bin/hack -action migrateGenesis -chaindata ${chaindata_dir} -input ${genesis_stub} -output genesis.json -ignore-scalable=true
