# How to run
```shell
./1-init-fep.sh
./2-update-rollupmgr.sh
./3-migrate-pp.sh
./4-restart-service.sh
```

# How to use bridge
```
http://127.0.0.1:8090/
L1 OKB Token: 0x5FbDB2315678afecb367f032d93F642f64180aa3
L2 WETH Token: 0x17a2a2e444a7f3446877d1b71eaa2b2ae7533baf
L2 admin: 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534


EOA1:
0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534
0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2

EOA2:
0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80

```

# How to use true prover
```
vim .env
PROVER_TYPE=true
```