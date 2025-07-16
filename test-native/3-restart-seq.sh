#!/bin/bash

# Strict mode: exit on command failure or undefined variable
set -eu
# set -x


docker-compose stop xlayer-seq
sleep 10
docker-compose up -d xlayer-seq