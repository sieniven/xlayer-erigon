#!/bin/bash
set -e
# set -x

#!/bin/bash
PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

#input=${1:-0}
#
## checking input
#if ! [[ "$input" =~ ^[0-9]$ ]]; then
#    echo "Error" >&2
#    exit 1
#fi

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}


if [ ! -d "./aggkit" ]; then
  echo "Cloning contract repository..."
  git clone -b feature/0.1.0 https://github.com/okx/aggkit.git
fi

cd ./aggkit
echo "Cleaning and resting contract repository..."
git reset --hard; git checkout feature/0.1.0;git pull

make build-docker


cd $PWD_DIR


DOCK_CONFIG_FILE="./docker-compose.yml"
sed_inplace "s|image: zjg555543/aggkit:upstream-v0.4.0-beta1|image: aggkit:local|g" "$DOCK_CONFIG_FILE"

#CDK_CONFIG_FILE="config/aggkit.toml"
#sed_inplace "s|\(LossCertType[[:space:]]*=[[:space:]]*\)[0-9]|\1$input|g" "$CDK_CONFIG_FILE"


source .env
if [ "$LOCAL_AGGLAYER" == "true" ]; then
  if [ ! -d "./agglayer" ]; then
    echo "Cloning contract repository..."
    git clone -b v0.3.3 https://github.com/agglayer/agglayer.git
  fi

  cd ./agglayer
  echo "Cleaning and resting agglayer repository..."

  CDK_CONFIG_FILE="Cargo.toml"
  sed_inplace 's|https://github.com/agglayer/provers.git|https://github.com/zjg555543/provers.git|g' "$CDK_CONFIG_FILE"
  sed_inplace 's|tag = "v1.1.1"|tag = "v1.1.1-host-1"|g' "$CDK_CONFIG_FILE"

  docker build -t agglayer:v0.3.3 -f Dockerfile .
  
  cd $PWD_DIR
  CDK_CONFIG_FILE="docker-compose.yml"
  sed_inplace "s|image: zjg555543/agglayer:upstream-v0.3.3|image: agglayer:v0.3.3|g" "$CDK_CONFIG_FILE"
fi
cd $PWD_DIR
docker-compose stop xlayer-aggkit
docker-compose up -d xlayer-aggkit
