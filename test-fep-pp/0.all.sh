#!/bin/bash
chmod +x *.sh

./1-init-fep.sh
./2-update-rollupmgr.sh
./3-update-pp.sh
./4-restart-service.sh