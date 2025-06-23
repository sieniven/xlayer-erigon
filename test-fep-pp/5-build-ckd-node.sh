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


if [ ! -d "./aggkit" ]; then
  echo "Cloning contract repository..."
  git clone -b zjg/fep-pp https://github.com/okx/aggkit.git
fi

cd ./aggkit
echo "Cleaning and resting contract repository..."
git reset --hard; git checkout zjg/fep-pp;git pull

make build-docker


cd $PWD_DIR


DOCK_CONFIG_FILE="./docker-compose.yml"
sed_inplace "s|image: zjg555543/aggkit:upstream-v0.4.0-beta1|image: aggkit:local|g" "$DOCK_CONFIG_FILE"

CDK_CONFIG_FILE="config/aggkit.toml"
sed_inplace "s|\(LossCertType[[:space:]]*=[[:space:]]*\)[0-9]|\1$input|g" "$CDK_CONFIG_FILE"

docker-compose stop xlayer-aggkit
docker-compose up -d xlayer-aggkit
