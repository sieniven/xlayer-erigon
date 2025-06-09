#!/bin/bash
chmod +x 1-init-fep.sh
chmod +x 2-update-rollupmgr.sh
chmod +x 3-update-pp.sh
chmod +x 4-restart-service.sh
chmod +x 5-build-ckd-node.sh
chmod +x 6-test-sig.sh

./1-init-fep.sh
./2-update-rollupmgr.sh
./3-update-pp.sh
./4-restart-service.sh