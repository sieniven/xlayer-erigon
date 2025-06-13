#!/bin/bash
chmod +x *.sh

./1-init-fep.sh
./2-1-upgrade-rollupmgr1.sh
./2-2-add-pp-type.sh
./2-3-update-rollupmgr2.sh
./3-update-pp.sh
./4-restart-service.sh