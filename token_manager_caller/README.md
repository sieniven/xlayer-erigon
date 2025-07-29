# TokenManagerCaller

1. Deploy

```sh
npx hardhat run deployUpgradeableTokenManager.js --network xlayer
```

2. Hardfork

```sh
make stop # do not remove the data
# set hardcode address of token manager caller proxy contract
make min-run
```

3. Test Permissions

```sh
cast send 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "grantOperatePermission(address)" 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534  -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --legacy --rpc-url http://127.0.0.1:8123

cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "canOperate(address)" 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --rpc-url http://127.0.0.1:8123

cast send 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "revokeOperatePermission(address)" 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534  -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --legacy --rpc-url http://127.0.0.1:8123

cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "canOperate(address)" 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --rpc-url http://127.0.0.1:8123
```

4. Test Mint

```sh
cast send 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "grantOperatePermission(address)" 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534  -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --legacy --rpc-url http://127.0.0.1:8123

cast balance 0x00000000000000000000000000000000000000ff --rpc-url localhost:8123

cast send 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "mint(address, uint256)" 0x00000000000000000000000000000000000000ff 1000000000000000000 -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --legacy --rpc-url http://127.0.0.1:8123

cast balance 0x00000000000000000000000000000000000000ff --rpc-url localhost:8123
```

5. Test Burn

```sh
cast send 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "grantBurnTargetPermission(address)" 0x00000000000000000000000000000000000000ff  -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --legacy --rpc-url http://127.0.0.1:8123

cast balance 0x00000000000000000000000000000000000000ff --rpc-url localhost:8123

cast send 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "burn(address, uint256)" 0x00000000000000000000000000000000000000ff 999999999999999999 -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --legacy --rpc-url http://127.0.0.1:8123

cast balance 0x00000000000000000000000000000000000000ff --rpc-url localhost:8123
```

6. Test Query Hardcoded Caller

```sh
cast call 0x0000000000000000000000000000000000000101 --data "0x20" --rpc-url http://127.0.0.1:8123
```

7. Test Upgrade

```sh
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "implementation()(address)" --rpc-url http://127.0.0.1:8123

npx hardhat run upgradeTokenManager.js --network xlayer

cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "canOperate(address)" 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534 --rpc-url http://127.0.0.1:8123
```