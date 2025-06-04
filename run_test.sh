#!/bin/sh

set -e

DOCKER_TEST_IMAGE="xlayer-erigon-test"
DOCKER_PARAM=("tests" "lint" "check_chinese_characters" "test-unwind-default" "test-unwind-split-db")
LOCAL_PARAM=("test-e2e" "test-data-loss" "test-executor" "kurtosis-cdk" "resequence-default" "resequence-ac-split")

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

function is_docker_param() {
    for param in "${DOCKER_PARAM[@]}"; do
        if [ "$1" == "$param" ]; then
            return 0
        fi
    done
    return 1
}

function is_local_param() {
    for param in "${LOCAL_PARAM[@]}"; do
        if [ "$1" == "$param" ]; then
            return 0
        fi
    done
    return 1
}

function cleanup() {
    if [ -f .dockerignore.bak ]; then
        mv .dockerignore.bak .dockerignore
    fi
}

function build_docker_image() {
    mv .dockerignore .dockerignore.bak
    trap cleanup EXIT      # normal exit
    trap cleanup SIGINT    # Ctrl+C
    trap cleanup SIGTERM   # kill 
    trap cleanup SIGQUIT   # Ctrl+\
    trap cleanup SIGHUP    # terminal shutdown

    create_dockerignore ".dockerignore"

    docker build -t $DOCKER_TEST_IMAGE -f ./Dockerfile.test .
    cleanup # clean up in advance
}

function run_in_docker() {
    docker run --rm $DOCKER_TEST_IMAGE $1
}

function test_all() {
    build_docker_image

    for param in "${DOCKER_PARAM[@]}"; do
        run_in_docker $param
    done
    for param in "${LOCAL_PARAM[@]}"; do
        cd ./test
        make $param
    done
}


cd $(dirname $0)

if [ "$1" == "all" ]; then
    test_all
elif is_local_param $1; then
    cd ./test
    make $1
elif is_docker_param $1; then
    build_docker_image
    run_in_docker $1
else
    echo "Error: invalid parameter '$1'"
    exit 1
fi


