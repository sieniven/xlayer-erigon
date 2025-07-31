# Token Manager Scripts 使用指南

## 📋 概述

本目录包含 Token Manager 系统的部署和测试脚本，提供完整的合约部署、配置和功能验证流程。

## 🚀 快速开始

```bash
# 1. 用Genesis的deploy账户给0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15转账
cast send 0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15 --value 10ether --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --rpc-url http://127.0.0.1:8123 --legacy

# 2. 部署合约
# 其中
#PROXY_ADMIN="0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15"   # 代理管理员，私钥是 0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9
#OWNER_ADDRESS="0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15"  # 合约Owner，私钥同上
#ADMIN_ADDRESS="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"  # 业务Admin，私钥是 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2
./deploy_tokenmanager.sh

# 3. 本地测试
./test_tokenmanager.sh

# 这样就可以进行完整的测试。
```

### 前置条件

1. **Erigon节点运行**: 确保 X Layer Erigon 节点正在运行
2. **RPC访问**: 确保可以访问节点的RPC端点 (默认: `http://localhost:8123`)
3. **私钥准备**: 准备部署账户的私钥
4. **Cast工具**: 确保已安装 Foundry 的 `cast` 命令行工具

```bash
# 安装 Foundry (如果尚未安装)
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

---

## 🛠️ 部署脚本

### `deploy_tokenmanager.sh`

完整的 Token Manager 系统部署脚本，包括实现合约和代理合约。

#### 基本用法

```bash
cd scripts
./deploy_tokenmanager.sh
```

#### 环境变量配置

```bash
# 基础配置
export PRIVATE_KEY="0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9"
export RPC_URL="http://localhost:8123"
export GAS_PRICE="1000000000"
export GAS_LIMIT="5000000"

# 权限分离配置
export PROXY_ADMIN="0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15"   # 代理管理员
export OWNER_ADDRESS="0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15"  # 合约Owner
export ADMIN_ADDRESS="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"  # 业务Admin

# 激活配置
export ACTIVATION_BLOCK="0"  # 立即激活

# 运行部署
./deploy_tokenmanager.sh
```

#### 部署流程

1. **编译合约** - 编译 TokenManagerV1 和 TokenManagerProxy
2. **部署实现合约** - 部署 TokenManagerV1 实现
3. **部署代理合约** - 部署 TransparentUpgradeableProxy
4. **初始化合约** - 设置 Owner、Admin 和激活区块
5. **验证部署** - 检查合约状态和权限

#### 输出信息

```
🎉 Token Manager System Deployed Successfully!
======================================

📊 Deployment Summary:
  Implementation: 0x1234...5678
  Proxy Address:  0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab
  Owner:         0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15
  Admin:         0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534
  Status:        Active (Block: 0)
```

---

## 🧪 测试脚本

### `test_tokenmanager.sh`

全面的 Token Manager 功能测试脚本，覆盖所有接口和边界情况。

#### 基本用法

```bash
cd scripts
./test_tokenmanager.sh
```

#### 测试配置

脚本使用硬编码的配置进行测试：

```bash
# 网络配置
RPC_URL="http://localhost:8123"
PROXY_ADDRESS="0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab"  # 固定的代理合约地址

# 测试账户私钥
OWNER_PRIVATE_KEY="0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9"
ADMIN_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"

# 测试金额
TEST_MINT_AMOUNT="1000000000000000000"    # 1 ETH
TEST_BURN_AMOUNT="500000000000000000"     # 0.5 ETH
ACCOUNT_FUNDING_AMOUNT="100000000000000000000"  # 100 ETH
```

#### 测试覆盖范围

| 测试类别 | 具体测试 |
|----------|----------|
| **基础查询** | VERSION, isActive, paused, activationBlock, owner, getAdmin |
| **角色管理** | grantMinterRole, grantBurnerRole, revokeMinterRole, revokeBurnerRole |
| **角色查询** | getMinterRoleCount, getBurnerRoleCount, getMintersPaginated, getBurnersPaginated |
| **白名单管理** | addMintWhitelist, removeMintWhitelist, getMintWhitelist, getBurnWhitelist |
| **白名单查询** | getMintWhitelistCount, getBurnWhitelistCount, isMintAllowed, isBurnAllowed |
| **核心功能** | mint, burn (成功和失败场景) |
| **权限控制** | 无权限操作被正确拒绝 |
| **暂停控制** | pause, unpause, 暂停状态下操作被拒绝 |
| **边界测试** | 零地址、零金额、重复操作、分页边界 |
| **错误参数** | 无效地址、无效操作 |
| **复杂场景** | 大量白名单、分页查询、并发权限检查 |

#### 测试流程

1. **创建临时账户** - 生成10个测试账户并分配资金
2. **基础功能测试** - 验证查询接口和状态
3. **权限管理测试** - 测试角色授予和撤销
4. **白名单测试** - 测试动态白名单管理
5. **核心操作测试** - 测试mint/burn功能
6. **暂停机制测试** - 测试pause/unpause
7. **边界情况测试** - 测试各种边界条件
8. **复杂场景测试** - 测试高级功能

#### 成功输出示例

```
🎉 Token Manager 全面测试完成
==============================

📊 测试覆盖范围:
  ✅ 基础查询接口
  ✅ 角色管理接口
  ✅ 白名单管理接口
  ✅ Mint/Burn功能
  ✅ 权限控制
  ✅ 暂停控制
  ✅ 边界情况测试
  ✅ 复杂场景测试

✅ 所有接口功能和边界情况测试通过！
```

---

## 🚨 系统限制

在使用脚本时，请注意以下系统限制：

| 限制项 | 数值 | 说明 |
|--------|------|------|
| **Mint白名单最大容量** | 500个地址 | 防止Gas耗尽(OOG) |
| **分页查询最大返回数** | 100个地址 | 防止单次查询OOG |
| **Burn余额保护** | 必须保留≥1 wei | 防止地址余额被完全清零 |

---

## 🛡️ 安全注意事项

1. **私钥保护**: 
   - 生产环境中不要在环境变量或命令行中暴露私钥
   - 建议使用硬件钱包或安全的密钥管理系统

2. **网络配置**:
   - 确保RPC_URL指向正确的网络
   - 验证合约地址是否正确

3. **权限分离**:
   - Owner控制系统级操作 (pause/unpause/setActivationBlock)
   - Admin控制业务级操作 (角色管理/白名单管理)
   - 确保权限分配符合安全原则

4. **测试环境**:
   - 在生产环境运行测试前，先在测试网验证
   - 测试脚本会创建临时账户和执行实际交易

---

## 📝 故障排除

### 常见问题

1. **"Connection refused"**
   ```bash
   # 检查Erigon节点是否运行
   curl -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        http://localhost:8123
   ```

2. **"insufficient funds"**
   ```bash
   # 检查部署账户余额
   cast balance $DEPLOYER_ADDRESS --rpc-url $RPC_URL
   ```

3. **"execution reverted"**
   ```bash
   # 检查合约是否正确部署
   cast code $PROXY_ADDRESS --rpc-url $RPC_URL
   ```

4. **"nonce too low/high"**
   ```bash
   # 重置账户nonce（如需要）
   cast nonce $ACCOUNT_ADDRESS --rpc-url $RPC_URL
   ```

### 调试模式

在脚本开头添加调试选项：

```bash
# 启用详细输出
set -x

# 在错误时停止
set -e
```

---

## 📚 相关文档

- [Token Manager API Reference](../docs/TokenManager_API_Reference.md)
- [Token Manager Complete Design](../docs/TokenManager_Complete_Design.md)
- [Contract Source Code](../contracts/TokenManagerV1.sol)

---

## 💬 支持

如遇到问题，请检查：
1. Erigon节点日志
2. 脚本输出的错误信息
3. 相关文档中的故障排除部分 