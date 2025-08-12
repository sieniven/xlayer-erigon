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
PROXY_ADDRESS="0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab"  # 代理合约地址
TARGET_ADDRESS="0x000000000000000000000000000000000000dEaD"  # 清理目标地址

# 测试账户私钥
OWNER_PRIVATE_KEY="0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9"
ADMIN_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
OPERATOR_PRIVATE_KEY="0x3c9229289a6125f7fdf1885a77bb12c37a8d3b4962d936f7e3084dece32a3ca1"

# 测试金额
BRIDGE_AMOUNT="1000000000000000000"    # 1 ETH
TRANSFER_AMOUNT="100000000000000000"  # 0.1 ETH (用于测试账户初始化)
```

#### 测试覆盖范围

| 测试类别 | 具体测试 |
|----------|----------|
| **操作员管理** | setOperator (支持零地址移除) |
| **管理员管理** | setAdmin |
| **角色查询** | operator(), admin() (自动生成getter) |
| **核心功能** | bridgeFrom (跨链到操作员), cleanup (清理目标地址) |
| **权限控制** | 无权限操作被正确拒绝，权限转移验证 |
| **暂停控制** | pause, unpause, 暂停状态下操作被拒绝 |
| **所有者管理** | transferOwnership, renounceOwnership (禁用) |
| **边界测试** | 重复cleanup操作、余额变化验证、零地址移除 |
| **系统查询** | VERSION(), isActive(), paused(), activationBlock() |

#### 测试流程

1. **Operator管理测试** - 设置、替换、移除(零地址)操作员
2. **BridgeFrom操作测试** - 测试代币跨链到操作员账户
3. **Cleanup操作测试** - 测试目标地址清理功能，验证1 wei保留机制
4. **暂停/恢复功能测试** - 测试pause/unpause机制
5. **查询功能测试** - 测试所有自动生成的getter接口
6. **Admin转移测试** - 测试管理员权限转移
7. **所有者转移测试** - 测试所有者权限转移

#### 核心接口变更

**移除的冗余接口**:
- ❌ `getAdmin()` → ✅ 使用 `admin()` (自动生成)
- ❌ `getCurrentOperator()` → ✅ 使用 `operator()` (自动生成)
- ❌ `removeOperator()` → ✅ 使用 `setOperator(address(0))`
- ❌ `hasAdmin()` / `hasOperator()` → ✅ 直接检查地址是否为零

**保留的核心接口**:
- ✅ `setAdmin(address)` - 设置新admin
- ✅ `setOperator(address)` - 设置operator (支持零地址移除)
- ✅ `bridgeFrom(uint256)` - 跨链代币到operator
- ✅ `cleanup()` - 清理目标地址到1 wei
- ✅ `pause()` / `unpause()` - 系统控制
- ✅ `transferOwnership(address)` - 所有权转移

#### 成功输出示例

```
🎉 所有测试完成!
  ✅ Operator管理正常
  ✅ BridgeFrom操作正常
  ✅ Cleanup操作正常
  ✅ 暂停/恢复功能正常
  ✅ 查询功能正常
  ✅ Admin转移功能正常
  ✅ 所有者转移功能正常
```

---

## 🏗️ 合约架构

### 地址控制模式

TokenManager V1 采用简化的地址控制模式，替代了复杂的角色控制：

```solidity
// 状态变量 (自动生成getter)
address public admin;     // admin() - 业务管理员
address public operator;  // operator() - 操作员

// 管理函数
function setAdmin(address newAdmin) external onlyAdmin;
function setOperator(address newOperator) external onlyAdmin;  // 支持address(0)移除
```

### 权限层级

```
Owner (系统级权限)
├── setActivationBlock()    - 设置激活区块
├── pause() / unpause()     - 暂停/恢复系统
└── transferOwnership()     - 转移所有权

Admin (业务级权限)
├── setAdmin()             - 设置新admin
└── setOperator()          - 设置/移除operator

Operator (操作级权限)
├── bridgeFrom()           - 跨链代币
└── cleanup()              - 清理目标地址
```

### 安全特性

1. **防抢跑保护**: 构造函数调用 `_disableInitializers()`
2. **防重入保护**: 所有状态变更函数使用 `nonReentrant`
3. **暂停机制**: 支持紧急暂停所有业务操作
4. **权限分离**: Owner/Admin/Operator 三级权限独立
5. **单地址控制**: 每个角色物理上只能有一个地址

---

## 🚨 系统限制

在使用脚本时，请注意以下系统限制：

| 限制项 | 数值 | 说明 |
|--------|------|------|
| **单一地址设计** | 每个角色只能有1个地址 | Admin/Operator采用单一地址模式，物理上无法设置多个 |
| **Cleanup余额保护** | 必须保留≥1 wei | 防止目标地址余额被完全清零 |
| **权限设计** | Owner/Admin完全独立 | 两个权限体系分离，但允许同一地址同时担任两个角色 |
| **接口简化** | 移除冗余函数 | 使用Solidity标准getter替代自定义查询函数 |

---

## 🛡️ 安全注意事项

1. **私钥保护**: 
   - 生产环境中不要在环境变量或命令行中暴露私钥
   - 建议使用硬件钱包或安全的密钥管理系统

2. **网络配置**:
   - 确保RPC_URL指向正确的网络
   - 验证合约地址是否正确

3. **权限分离**:
   - Owner控制系统级操作 (pause/unpause/transferOwnership)
   - Admin控制业务级操作 (setAdmin/setOperator)
   - Operator执行业务操作 (bridgeFrom/cleanup)
   - 确保权限分配符合安全原则

4. **测试环境**:
   - 在生产环境运行测试前，先在测试网验证
   - 测试脚本会自动为测试账户提供资金并执行实际交易
   - 基础状态验证已移至部署脚本，测试脚本专注于功能测试

5. **接口调用**:
   - 使用 `admin()` 和 `operator()` 替代已移除的getter函数
   - 使用 `setOperator(address(0))` 移除operator
   - 注意Owner和Admin权限完全独立

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

5. **"function not found"**
   ```bash
   # 检查是否使用了已移除的函数
   # 使用 admin() 替代 getAdmin()
   # 使用 operator() 替代 getCurrentOperator()
   # 使用 setOperator(address(0)) 替代 removeOperator()
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
- [Token Manager V2 Upgrade Feasibility](../docs/TokenManager_V2_Upgrade_Feasibility.md)
- [Contract Source Code](../contracts/TokenManagerV1.sol)

---

## 💬 支持

如遇到问题，请检查：
1. Erigon节点日志
2. 脚本输出的错误信息
3. 相关文档中的故障排除部分
4. 确保使用正确的接口名称 (admin/operator getter)