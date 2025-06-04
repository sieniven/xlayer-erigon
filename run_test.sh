#!/bin/sh

function create_dockerignore() {
    ignorefile=$1
    echo "**/*.a" > $ignorefile
    echo "**/*.dylib" >> $ignorefile
    echo "**/*.o" >> $ignorefile
    echo "**/*.dSYM" >> $ignorefile
    echo "build" >> $ignorefile
    echo "cmd/prometheus" >> $ignorefile
    echo "vendor" >> $ignorefile
    echo "cache.db" >> $ignorefile
    echo ".git" >> $ignorefile
    echo "mainnet" >> $ignorefile
}

# change to build directory to avoid using .dockerignore in the root directory
mkdir -p build && cd build

create_dockerignore ".dockerignore"

docker build -t xlayer-erigon-test -f ../Dockerfile.test ..
docker run --rm -it xlayer-erigon-test

cd ..


