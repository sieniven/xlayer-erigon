# Token Manager API 接口文档

## 📋 概述

Token Manager V1 是一个可升级的代币管理系统，提供安全的代币铸造(mint)和销毁(burn)功能。基于 OpenZeppelin 标准合约构建，具有权限控制、暂停机制和升级能力。

**合约地址**: 通过 TokenManagerProxy 代理合约访问  
**Precompile地址**: `0x0000000000000000000000000000000000008888`  
**版本**: v1.0.0

---

## 🔧 核心功能接口

### 代币操作

#### `mint(address to, uint256 amount)`
铸造代币到指定地址

**权限**: 仅管理员(onlyOwner)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile)

**参数**:
- `to`: 接收代币的地址 (不能为零地址)
- `amount`: 铸造数量 (Wei, 必须大于0)

**事件**: `TokenMinted(address indexed to, uint256 amount)`

**调用示例**:
```bash
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "mint(address,uint256)" $TO_ADDRESS $AMOUNT --legacy
```

#### `burn(address from, uint256 amount)`
从指定地址销毁代币

**权限**: 仅管理员(onlyOwner)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile)

**参数**:
- `from`: 销毁代币的地址 (不能为零地址，必须在白名单中)
- `amount`: 销毁数量 (Wei, 必须大于0，不能销毁全部余额)

**限制**:
- 地址必须在burn白名单中
- 不能销毁地址的全部余额

**事件**: `TokenBurned(address indexed from, uint256 amount)`

**调用示例**:
```bash
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "burn(address,uint256)" $FROM_ADDRESS $AMOUNT --legacy
```

#### `batchMint(address[] recipients, uint256[] amounts)`
批量铸造代币到多个地址

**权限**: 仅管理员(onlyOwner)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile)

**参数**:
- `recipients`: 接收地址数组
- `amounts`: 对应的铸造数量数组

**限制**:
- 两个数组长度必须相等
- 所有地址不能为零地址
- 所有数量必须大于0

---

## 🔐 权限管理

### 管理员操作

#### `owner() → address`
获取当前合约管理员地址

**权限**: 公开查询  
**返回**: 当前管理员地址

#### `transferOwnership(address newOwner)`
转移管理员权限 (OpenZeppelin 标准)

**权限**: 仅管理员(onlyOwner)  
**参数**: `newOwner` - 新管理员地址

#### `renounceOwnership()`
放弃管理员权限 (已禁用)

**权限**: 已禁用  
**状态**: 调用会直接失败  
**错误**: `"TokenManager: renounceOwnership is disabled for security"`  
**说明**: 为防止合约变成不可管理状态，此功能已被禁用

#### `getAdmin() → address`
获取管理员地址 (兼容性接口)

**权限**: 公开查询  
**返回**: 当前管理员地址 (同owner())

---

## 📋 白名单管理

#### `addBurnWhitelist(address account)`
添加地址到burn白名单

**权限**: 仅管理员(onlyOwner)  
**参数**: `account` - 要添加的地址 (支持零地址)  
**事件**: `BurnWhitelistAdded(address indexed account)`

#### `removeBurnWhitelist(address account)`
从burn白名单移除地址

**权限**: 仅管理员(onlyOwner)  
**参数**: `account` - 要移除的地址  
**事件**: `BurnWhitelistRemoved(address indexed account)`

#### `batchAddBurnWhitelist(address[] accounts)`
批量添加地址到burn白名单

**权限**: 仅管理员(onlyOwner)  
**参数**: `accounts` - 地址数组

#### `getBurnWhitelist() → address[]`
获取所有burn白名单地址

**权限**: 公开查询  
**返回**: 白名单地址数组

#### `getBurnWhitelistCount() → uint256`
获取白名单地址数量

**权限**: 公开查询  
**返回**: 白名单中的地址数量

#### `burnWhitelist(address) → bool`
检查地址是否在白名单中

**权限**: 公开查询  
**参数**: 要检查的地址  
**返回**: true(在白名单) / false(不在白名单)

#### `isBurnAllowed(address account) → bool`
检查地址是否允许burn操作

**权限**: 公开查询  
**参数**: 要检查的地址  
**返回**: true(允许) / false(不允许)  
**注意**: 如果白名单为空，默认允许所有地址(向后兼容)

---

## ⚡ 系统状态管理

### 激活控制

#### `setActivationBlock(uint256 activationBlock)`
设置合约激活区块

**权限**: 仅管理员(onlyOwner)  
**参数**: `activationBlock` - 激活区块号  
**事件**: `ActivationBlockSet(uint256 activationBlock)`

#### `activationBlock() → uint256`
获取激活区块号

**权限**: 公开查询  
**返回**: 激活区块号

#### `isActive() → bool`
检查合约是否已激活

**权限**: 公开查询  
**返回**: true(已激活) / false(未激活)  
**条件**: 当前区块 >= 激活区块 且 有有效管理员

### 暂停控制

#### `pause()`
暂停合约 (紧急停止)

**权限**: 仅管理员(onlyOwner)  
**效果**: 阻止所有mint/burn操作  
**事件**: `Paused(address account)`

#### `unpause()`
恢复合约运行

**权限**: 仅管理员(onlyOwner)  
**效果**: 恢复所有操作  
**事件**: `Unpaused(address account)`

#### `paused() → bool`
查询合约是否已暂停

**权限**: 公开查询  
**返回**: true(已暂停) / false(正常运行)

---

## 🔍 系统诊断

#### `isPrecompileAvailable() → bool`
检查Precompile是否可用

**权限**: 公开查询  
**返回**: true(可用) / false(不可用)  
**原理**: 调用Precompile的TEST_OP操作

#### `VERSION() → string`
获取合约版本

**权限**: 公开查询  
**返回**: 版本字符串 "v1.0.0"

---

## 📤 事件列表

```solidity
// 初始化事件
event Initialized(address indexed owner, uint256 activationBlock);

// 激活控制事件
event ActivationBlockSet(uint256 activationBlock);

// 白名单管理事件
event BurnWhitelistAdded(address indexed account);
event BurnWhitelistRemoved(address indexed account);

// 代币操作事件
event TokenMinted(address indexed to, uint256 amount);
event TokenBurned(address indexed from, uint256 amount);

// OpenZeppelin 标准事件
event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
event Paused(address account);
event Unpaused(address account);
```

---

## 🛡️ 访问控制修饰符

- **`onlyOwner`**: 仅管理员可调用
- **`onlyActive`**: 合约必须已激活
- **`whenNotPaused`**: 合约必须未暂停
- **`onlyWithPrecompile`**: Precompile必须可用

---

## 🚨 错误处理

### 常见错误信息

- `"Ownable: caller is not the owner"` - 非管理员调用
- `"Token Manager is not active"` - 合约未激活
- `"Pausable: paused"` - 合约已暂停
- `"Precompile is not available"` - Precompile不可用
- `"Cannot mint to zero address"` - 铸造到零地址
- `"Amount must be greater than zero"` - 数量必须大于0
- `"Address is not in burn whitelist"` - 地址不在白名单
- `"Cannot burn entire balance"` - 不能销毁全部余额
- `"Address is already whitelisted"` - 地址已在白名单
- `"Address is not whitelisted"` - 地址不在白名单
- `"TokenManager: renounceOwnership is disabled for security"` - 禁用放弃所有权操作

---

## 📝 使用示例

### 初始化部署
```bash
# 1. 部署并初始化
./scripts/deploy_tokenmanager.sh

# 2. 设置激活区块 (立即激活)
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "setActivationBlock(uint256)" 0 --legacy
```

### 权限管理
```bash
# 转移管理员权限 (推荐使用)
cast send --private-key $CURRENT_ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "transferOwnership(address)" $NEW_ADMIN_ADDRESS --legacy

# 查询当前管理员
cast call --rpc-url $RPC $PROXY_ADDRESS "owner()"

# ❌ 禁用操作 - 会失败
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "renounceOwnership()" --legacy
# Error: TokenManager: renounceOwnership is disabled for security
```

### 白名单管理
```bash
# 添加burn白名单
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "addBurnWhitelist(address)" $ADDRESS --legacy

# 查询白名单
cast call --rpc-url $RPC $PROXY_ADDRESS "getBurnWhitelist()"

# 检查地址是否可burn
cast call --rpc-url $RPC $PROXY_ADDRESS \
  "isBurnAllowed(address)" $ADDRESS
```

### 代币操作
```bash
# 铸造10个代币 (10 * 10^18 Wei)
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "mint(address,uint256)" $TO_ADDRESS 10000000000000000000 --legacy

# 销毁5个代币
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "burn(address,uint256)" $FROM_ADDRESS 5000000000000000000 --legacy
```

### 紧急控制
```bash
# 暂停合约
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "pause()" --legacy

# 恢复合约
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "unpause()" --legacy
```

---

## ⚠️ 重要注意事项

1. **权限管理**: 管理员权限非常重要，务必安全保管私钥
2. **激活机制**: 合约必须先激活才能进行mint/burn操作
3. **白名单限制**: burn操作需要地址在白名单中
4. **余额保护**: 不能销毁地址的全部余额
5. **暂停功能**: 可用于紧急情况下停止所有操作
6. **升级能力**: 通过代理合约支持未来升级
7. **所有权安全**: `renounceOwnership()` 已被禁用，防止合约失去管理员而变得不可用

---

## 🔗 相关文档

- [完整设计文档](TokenManager_Complete_Design.md)
- [部署脚本使用说明](../scripts/deploy_tokenmanager.sh)
- [OpenZeppelin Contracts 文档](https://docs.openzeppelin.com/contracts/) 