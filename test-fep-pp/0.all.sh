#!/bin/bash
set -e

chmod +x *.sh

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

# Check if parameter is provided
if [ "$1" = "1" ]; then
    echo "Executing stress test flow..."
    sed_inplace 's| - "1"| - "12"|g' "docker-compose.yml"
    sed_inplace 's|sleep 20|sleep 299|g' "1-init-fep.sh"
    sed_inplace 's|./6-bridge.sh|echo skip bridge|g' "1-init-fep.sh"
    ./1-init-fep.sh
    ./8-bridge-stress.sh 10000
    ./2-update-rollupmgr.sh
    ./3-update-pp.sh
    ./4-restart-service.sh
else
    echo "Executing normal flow..."
    sed_inplace 's| - "12"| - "1"|g' "docker-compose.yml"
    sed_inplace 's|sleep 299|sleep 20|g' "1-init-fep.sh"
    sed_inplace 's|echo skip bridge|./6-bridge.sh|g' "1-init-fep.sh"
    ./1-init-fep.sh
    ./2-update-rollupmgr.sh
    ./3-update-pp.sh
    ./4-restart-service.sh
fi
