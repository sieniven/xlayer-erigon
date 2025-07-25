#!/bin/bash
set -e

chmod +x *.sh

./1-init-fep.sh
./8-bridge-stress.sh 1000
./2-update-rollupmgr.sh
./3-update-pp.sh
./4-restart-service.sh
