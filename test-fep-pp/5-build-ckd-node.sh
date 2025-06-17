#!/bin/bash
set -e
# set -x

#!/bin/bash
PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

input=${1:-0}

# checking input
if ! [[ "$input" =~ ^[0-9]$ ]]; then
    echo "Error" >&2
    exit 1
fi

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}


if [ ! -d "./xlayer-cdk" ]; then
  echo "Cloning contract repository..."
  git clone -b zjg/v0.5.4-rc1 https://github.com/okx/xlayer-cdk.git
fi

cd ./xlayer-cdk
echo "Cleaning and resting contract repository..."
git reset --hard; git checkout zjg/v0.5.4-rc1;git pull

make build-docker


cd $PWD_DIR


DOCK_CONFIG_FILE="./docker-compose.yml"
sed_inplace "s|image: zjg555543/cdk:v0.5.4-rc1|image: cdk|g" "$DOCK_CONFIG_FILE"

CDK_CONFIG_FILE="config/cdk-node-config.toml"
sed_inplace "s|\(LossCertType[[:space:]]*=[[:space:]]*\)[0-9]|\1$input|g" "$CDK_CONFIG_FILE"

docker-compose stop xlayer-cdk-node
docker-compose up -d xlayer-cdk-node
