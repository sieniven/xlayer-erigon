#!/bin/bash
set -e
# set -x

if [ ! -d "./xlayer-cdk" ]; then
  echo "Cloning contract repository..."
  git clone -b upstream/v0.5.4-rc1 https://github.com/okx/xlayer-cdk.git
fi

cd ./xlayer-cdk
echo "Cleaning and resting contract repository..."
rm -rf *; git reset --hard; git checkout upstream/v0.5.4-rc1
