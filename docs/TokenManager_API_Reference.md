# Token Manager API 接口文档

## 📋 概述

Token Manager V1 是一个可升级的代币管理系统，提供安全的代币铸造(mint)和销毁(burn)功能。基于 OpenZeppelin 标准合约构建，具有权限控制、暂停机制和升级能力。

**合约地址**: 通过 TokenManagerProxy 代理合约访问  
**Precompile地址**: `0x0000000000000000000000000000000000008888`  
**版本**: v1.0.0

---

## 🔐 角色权限表

### 详细接口权限分配

| 角色 | 角色标识符 | 权限说明 | 专有接口 (仅此角色可调用) |
|------|------------|----------|--------------------------|
| **ADMIN_ROLE** | `0x0000000000000000000000000000000000000000000000000000000000000000` | 合约管理员 | **角色管理**:<br/>• `grantMinterRole(address)`<br/>• `revokeMinterRole(address)`<br/>• `grantBurnerRole(address)`<br/>• `revokeBurnerRole(address)`<br/>• `getMinterRoleCount()`<br/>• `getBurnerRoleCount()`<br/>• `getAllMinters()`<br/>• `getAllBurners()`<br/>• `getMintersPaginated(uint256,uint256)`<br/>• `getBurnersPaginated(uint256,uint256)`<br/><br/>**白名单管理**:<br/>• `addMintWhitelist(address)`<br/>• `removeMintWhitelist(address)`<br/>• `batchAddMintWhitelist(address[])`<br/>• `addBurnWhitelist(address)`<br/>• `removeBurnWhitelist(address)`<br/>• `batchAddBurnWhitelist(address[])`<br/><br/>**系统控制**:<br/>• `setActivationBlock(uint256)`<br/>• `pause()`<br/>• `unpause()`<br/>• `transferOwnership(address)` |
| **MINTER_ROLE** | `keccak256("MINTER_ROLE")` | 铸造操作员 | **铸造操作**:<br/>• `mint(address,uint256)`<br/>• `batchMint(address[],uint256[])` |
| **BURNER_ROLE** | `keccak256("BURNER_ROLE")` | 销毁操作员 | **销毁操作**:<br/>• `burn(address,uint256)`<br/>• `batchBurn(address[],uint256[])` |
| **Owner** | 合约所有者 | 继承ADMIN_ROLE | • **自动拥有ADMIN_ROLE的所有专有接口**<br/>• **不自动拥有MINTER_ROLE或BURNER_ROLE** |

### 公开查询接口 (所有用户可调用)

| 接口类型 | 具体接口 |
|----------|----------|
| **基础信息** | • `owner()` - 获取Owner地址 *(自动生成)*<br/>• `getAdmin()` - 获取管理员地址<br/>• `VERSION()` - 获取版本信息<br/>• `hasRole(bytes32,address)` - 检查角色权限 *(自动生成)* |
| **系统状态** | • `isActive()` - 检查激活状态<br/>• `activationBlock()` - 获取激活区块 *(自动生成)*<br/>• `paused()` - 检查暂停状态 *(自动生成)*<br/>• `isPrecompileAvailable()` - 检查Precompile可用性 |
| **常量查询** | • `MAX_BATCH_SIZE()` - 批量操作限制 *(自动生成)*<br/>• `MAX_WHITELIST_RETURN()` - 分页查询限制 *(自动生成)*<br/>• `ADMIN_ROLE()` - 管理员角色标识符 *(自动生成)*<br/>• `MINTER_ROLE()` - 铸造者角色标识符 *(自动生成)*<br/>• `BURNER_ROLE()` - 销毁者角色标识符 *(自动生成)* |
| **白名单查询** | • `getMintWhitelist(uint256,uint256)` - 分页获取mint白名单<br/>• `getMintWhitelistCount()` - 获取mint白名单数量<br/>• `mintWhitelist(address)` - 检查mint白名单状态 *(自动生成)*<br/>• `isMintAllowed(address)` - 检查mint权限<br/>• `getBurnWhitelist(uint256,uint256)` - 分页获取burn白名单<br/>• `getBurnWhitelistCount()` - 获取burn白名单数量<br/>• `burnWhitelist(address)` - 检查burn白名单状态 *(自动生成)*<br/>• `isBurnAllowed(address)` - 检查burn权限 |

**重要说明**:
- 合约Owner默认只拥有ADMIN_ROLE，**不会自动获得MINTER_ROLE或BURNER_ROLE**
- 要进行mint/burn操作，必须明确授予相应角色
- 每个角色可以授予给多个地址
- ADMIN_ROLE可以管理其他所有角色
- 转移Owner时，旧Owner的ADMIN_ROLE会被撤销，新Owner自动获得ADMIN_ROLE

---

## 🔧 核心功能接口

### 代币操作

#### `mint(address to, uint256 amount)`
铸造代币到指定地址

**权限**: 仅Minter角色(onlyRole(MINTER_ROLE)) + 重入保护(nonReentrant)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile) + 目标地址在白名单(onlyMintWhitelisted)

**参数**:
- `to`: 接收代币的地址 (不能为零地址，必须在mint白名单中)
- `amount`: 铸造数量 (Wei, 必须大于0)

**事件**: `TokenMinted(address indexed to, uint256 amount, address indexed minter)`

**调用示例**:
```bash
cast send --private-key $MINTER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "mint(address,uint256)" $TO_ADDRESS $AMOUNT --legacy
```

#### `burn(address from, uint256 amount)`
从指定地址销毁代币

**权限**: 仅Burner角色(onlyRole(BURNER_ROLE)) + 重入保护(nonReentrant)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile) + 源地址在白名单(onlyBurnWhitelisted)

**参数**:
- `from`: 销毁代币的地址 (不能为零地址，必须在burn白名单中)
- `amount`: 销毁数量 (Wei, 必须大于0，不能销毁全部余额)

**限制**:
- 地址必须在burn白名单中
- 不能销毁地址的全部余额

**事件**: `TokenBurned(address indexed from, uint256 amount, address indexed burner)`

**调用示例**:
```bash
cast send --private-key $BURNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "burn(address,uint256)" $FROM_ADDRESS $AMOUNT --legacy
```

#### `batchMint(address[] recipients, uint256[] amounts)`
批量铸造代币到多个地址

**权限**: 仅Minter角色(onlyRole(MINTER_ROLE)) + 重入保护(nonReentrant)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile)

**参数**:
- `recipients`: 接收地址数组
- `amounts`: 对应的铸造数量数组

**限制**:
- 两个数组长度必须相等
- 所有地址不能为零地址
- 所有数量必须大于0
- 所有地址必须在mint白名单中
- 数组长度不能超过20个 (MAX_BATCH_SIZE)

**事件**: `BatchTokenMinted(address[] indexed recipients, uint256[] amounts, address indexed minter)`

#### `batchBurn(address[] sources, uint256[] amounts)`
批量销毁多个地址的代币

**权限**: 仅Burner角色(onlyRole(BURNER_ROLE)) + 重入保护(nonReentrant)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile)

**参数**:
- `sources`: 销毁代币的地址数组
- `amounts`: 对应的销毁数量数组

**限制**:
- 两个数组长度必须相等
- 所有地址不能为零地址
- 所有数量必须大于0
- 所有地址必须在burn白名单中
- 不能销毁任何地址的全部余额
- 数组长度不能超过20个 (MAX_BATCH_SIZE)

**事件**: `BatchTokenBurned(address[] indexed sources, uint256[] amounts, address indexed burner)`

---

## 🔐 权限管理

### 管理员操作

#### `owner() → address`
获取当前合约Owner地址

**权限**: 公开查询  
**返回**: 当前Owner地址 (拥有ADMIN_ROLE)

#### `transferOwnership(address newOwner)`
转移Owner权限 (安全转移)

**权限**: 仅Owner(onlyOwner)  
**参数**: `newOwner` - 新Owner地址 (不能为零地址)  
**逻辑**: 
1. 撤销旧Owner的ADMIN_ROLE
2. 授予新Owner的ADMIN_ROLE  
3. 转移所有权

**事件**: `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)`

#### `renounceOwnership()`
放弃Owner权限 (已禁用)

**权限**: 已禁用  
**状态**: 调用会直接失败  
**错误**: `"TokenManager: renounceOwnership is disabled for security"`  
**说明**: 为防止合约变成不可管理状态，此功能已被禁用

#### `getAdmin() → address`
获取管理员地址 (兼容性接口)

**权限**: 公开查询  
**返回**: 当前Owner地址 (同owner())

### 角色管理

#### `grantMinterRole(address account)`
授予Minter角色

**权限**: 仅Owner(onlyOwner)  
**参数**: `account` - 要授予角色的地址  
**事件**: `MinterRoleGranted(address indexed account, address indexed sender)`

#### `revokeMinterRole(address account)`
撤销Minter角色

**权限**: 仅Owner(onlyOwner)  
**参数**: `account` - 要撤销角色的地址  
**事件**: `MinterRoleRevoked(address indexed account, address indexed sender)`

#### `grantBurnerRole(address account)`
授予Burner角色

**权限**: 仅Owner(onlyOwner)  
**参数**: `account` - 要授予角色的地址  
**事件**: `BurnerRoleGranted(address indexed account, address indexed sender)`

#### `revokeBurnerRole(address account)`
撤销Burner角色

**权限**: 仅Owner(onlyOwner)  
**参数**: `account` - 要撤销角色的地址  
**事件**: `BurnerRoleRevoked(address indexed account, address indexed sender)`

#### `hasRole(bytes32 role, address account) → bool`
检查地址是否拥有指定角色 (OpenZeppelin 标准)

**权限**: 公开查询  
**参数**: 
- `role` - 角色标识符 (ADMIN_ROLE/MINTER_ROLE/BURNER_ROLE)
- `account` - 要检查的地址  
**返回**: true(拥有角色) / false(没有角色)

### 角色查询 (基于 AccessControlEnumerable)

#### `getMinterRoleCount() → uint256`
获取拥有Minter角色的地址数量

**权限**: 仅Owner(onlyOwner)  
**返回**: Minter角色成员数量

#### `getBurnerRoleCount() → uint256`
获取拥有Burner角色的地址数量

**权限**: 仅Owner(onlyOwner)  
**返回**: Burner角色成员数量

#### `getAllMinters() → address[]`
获取所有拥有Minter角色的地址

**权限**: 仅Owner(onlyOwner)  
**返回**: Minter地址数组

#### `getAllBurners() → address[]`
获取所有拥有Burner角色的地址

**权限**: 仅Owner(onlyOwner)  
**返回**: Burner地址数组

#### `getMintersPaginated(uint256 offset, uint256 limit) → address[]`
分页获取Minter角色地址

**权限**: 仅Owner(onlyOwner)  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量  
**返回**: 分页的Minter地址数组

#### `getBurnersPaginated(uint256 offset, uint256 limit) → address[]`
分页获取Burner角色地址

**权限**: 仅Owner(onlyOwner)  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量  
**返回**: 分页的Burner地址数组

---

## 📋 白名单管理

### Mint白名单

#### `addMintWhitelist(address account)`
添加地址到mint白名单

**权限**: 仅Owner(onlyOwner)  
**参数**: `account` - 要添加的地址  
**事件**: `MintWhitelistAdded(address indexed account, address indexed sender)`

#### `removeMintWhitelist(address account)`
从mint白名单移除地址

**权限**: 仅Owner(onlyOwner)  
**参数**: `account` - 要移除的地址  
**事件**: `MintWhitelistRemoved(address indexed account, address indexed sender)`

#### `batchAddMintWhitelist(address[] accounts)`
批量添加地址到mint白名单

**权限**: 仅Owner(onlyOwner)  
**参数**: `accounts` - 地址数组 (最多20个)

#### `getMintWhitelist(uint256 offset, uint256 limit) → (address[] addresses, uint256 total)`
分页获取mint白名单地址

**权限**: 公开查询  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量 (最多100个，受MAX_WHITELIST_RETURN限制)  
**返回**: 地址数组和总数

#### `getMintWhitelistCount() → uint256`
获取mint白名单地址数量

**权限**: 公开查询  
**返回**: 白名单中的地址数量

#### `mintWhitelist(address) → bool`
直接查询地址在mint白名单映射中的状态

**权限**: 公开查询  
**参数**: 要检查的地址  
**返回**: true(在白名单映射中) / false(不在白名单映射中)  
**注意**: 这是自动生成的public mapping getter函数

#### `isMintAllowed(address account) → bool`
检查地址是否真正允许mint操作 (推荐使用)

**权限**: 公开查询  
**参数**: 要检查的地址  
**返回**: true(允许) / false(不允许)  
**逻辑**: 
1. 如果白名单为空 → 返回false (安全优先)
2. 如果白名单不为空 → 返回 `mintWhitelist[account]`

**与mintWhitelist(address)的区别**:
- `mintWhitelist(address)`: 只查询映射值，不考虑白名单是否为空
- `isMintAllowed(address)`: 包含"空白名单默认拒绝"的安全逻辑

### Burn白名单

#### `addBurnWhitelist(address account)`
添加地址到burn白名单

**权限**: 仅Owner(onlyOwner)  
**参数**: `account` - 要添加的地址 (支持零地址)  
**事件**: `BurnWhitelistAdded(address indexed account, address indexed sender)`

#### `removeBurnWhitelist(address account)`
从burn白名单移除地址

**权限**: 仅Owner(onlyOwner)  
**参数**: `account` - 要移除的地址  
**事件**: `BurnWhitelistRemoved(address indexed account, address indexed sender)`

#### `batchAddBurnWhitelist(address[] accounts)`
批量添加地址到burn白名单

**权限**: 仅Owner(onlyOwner)  
**参数**: `accounts` - 地址数组 (最多20个)

#### `getBurnWhitelist(uint256 offset, uint256 limit) → (address[] addresses, uint256 total)`
分页获取burn白名单地址

**权限**: 公开查询  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量 (最多100个，受MAX_WHITELIST_RETURN限制)  
**返回**: 地址数组和总数

#### `getBurnWhitelistCount() → uint256`
获取burn白名单地址数量

**权限**: 公开查询  
**返回**: 白名单中的地址数量

#### `burnWhitelist(address) → bool`
直接查询地址在burn白名单映射中的状态

**权限**: 公开查询  
**参数**: 要检查的地址  
**返回**: true(在白名单映射中) / false(不在白名单映射中)  
**注意**: 这是自动生成的public mapping getter函数

#### `isBurnAllowed(address account) → bool`
检查地址是否真正允许burn操作 (推荐使用)

**权限**: 公开查询  
**参数**: 要检查的地址  
**返回**: true(允许) / false(不允许)  
**逻辑**: 
1. 如果白名单为空 → 返回false (安全优先)
2. 如果白名单不为空 → 返回 `burnWhitelist[account]`

**与burnWhitelist(address)的区别**:
- `burnWhitelist(address)`: 只查询映射值，不考虑白名单是否为空
- `isBurnAllowed(address)`: 包含"空白名单默认拒绝"的安全逻辑

---

## ⚡ 系统状态管理

### 激活控制

#### `setActivationBlock(uint256 activationBlock)`
设置合约激活区块

**权限**: 仅Owner(onlyOwner)  
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

**权限**: 仅Owner(onlyOwner)  
**效果**: 阻止所有mint/burn操作  
**事件**: `ContractPaused(address indexed account)`

#### `unpause()`
恢复合约运行

**权限**: 仅Owner(onlyOwner)  
**效果**: 恢复所有操作  
**事件**: `ContractUnpaused(address indexed account)`

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
**原理**: 调用Precompile的TEST_OP操作并检查返回值

#### `VERSION() → string`
获取合约版本

**权限**: 公开查询  
**返回**: 版本字符串 "v1.0.0"

#### 常量查询

#### `MAX_BATCH_SIZE() → uint256`
获取批量操作最大限制

**权限**: 公开查询  
**返回**: 20 (批量操作的最大数组长度)

#### `MAX_WHITELIST_RETURN() → uint256`
获取分页查询最大返回数量

**权限**: 公开查询  
**返回**: 100 (分页查询的最大返回数量)

#### `ADMIN_ROLE() → bytes32`
获取管理员角色标识符

**权限**: 公开查询  
**返回**: `0x0000000000000000000000000000000000000000000000000000000000000000`

#### `MINTER_ROLE() → bytes32`
获取铸造者角色标识符

**权限**: 公开查询  
**返回**: `keccak256("MINTER_ROLE")`

#### `BURNER_ROLE() → bytes32`
获取销毁者角色标识符

**权限**: 公开查询  
**返回**: `keccak256("BURNER_ROLE")`

---

## 📤 事件列表

### 系统事件
```solidity
// 初始化事件
event Initialized(address indexed owner, uint256 activationBlock);

// 激活控制事件
event ActivationBlockSet(uint256 activationBlock);

// 系统控制事件
event ContractPaused(address indexed account);
event ContractUnpaused(address indexed account);
```

### 角色管理事件
```solidity
// 角色管理事件
event MinterRoleGranted(address indexed account, address indexed sender);
event MinterRoleRevoked(address indexed account, address indexed sender);
event BurnerRoleGranted(address indexed account, address indexed sender);
event BurnerRoleRevoked(address indexed account, address indexed sender);
```

### 白名单管理事件
```solidity
// 白名单管理事件
event MintWhitelistAdded(address indexed account, address indexed sender);
event MintWhitelistRemoved(address indexed account, address indexed sender);
event BurnWhitelistAdded(address indexed account, address indexed sender);
event BurnWhitelistRemoved(address indexed account, address indexed sender);
```

### 代币操作事件
```solidity
// 代币操作事件
event TokenMinted(address indexed to, uint256 amount, address indexed minter);
event TokenBurned(address indexed from, uint256 amount, address indexed burner);
event BatchTokenMinted(address[] indexed recipients, uint256[] amounts, address indexed minter);
event BatchTokenBurned(address[] indexed sources, uint256[] amounts, address indexed burner);
```

### OpenZeppelin 标准事件
```solidity
// OpenZeppelin 标准事件
event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
event Paused(address account);
event Unpaused(address account);
event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender);
event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender);
```

---

## 🛡️ 访问控制修饰符

- **`onlyOwner`**: 仅Owner可调用
- **`onlyRole(ADMIN_ROLE)`**: 仅Admin角色可调用
- **`onlyRole(MINTER_ROLE)`**: 仅Minter角色可调用
- **`onlyRole(BURNER_ROLE)`**: 仅Burner角色可调用
- **`onlyActive`**: 合约必须已激活
- **`whenNotPaused`**: 合约必须未暂停
- **`onlyWithPrecompile`**: Precompile必须可用
- **`nonReentrant`**: 重入保护
- **`onlyMintWhitelisted(address)`**: 地址必须在mint白名单
- **`onlyBurnWhitelisted(address)`**: 地址必须在burn白名单

---

## 🚨 错误处理

### 常见错误信息

- `"OwnableUnauthorizedAccount"` - 非Owner调用仅Owner函数
- `"AccessControlUnauthorizedAccount"` - 账户缺少必要角色
- `"Token Manager is not active"` - 合约未激活
- `"EnforcedPause"` - 合约已暂停
- `"Precompile is not available"` - Precompile不可用
- `"Cannot mint to zero address"` - 铸造到零地址
- `"Cannot burn from zero address"` - 从零地址销毁
- `"Amount must be greater than zero"` - 数量必须大于0
- `"Address is not in mint whitelist"` - 地址不在mint白名单
- `"Address is not in burn whitelist"` - 地址不在burn白名单
- `"Cannot burn entire balance"` - 不能销毁全部余额
- `"Address is already in mint whitelist"` - 地址已在mint白名单
- `"Address is already in burn whitelist"` - 地址已在burn白名单
- `"Too many recipients"` - 批量操作超过数量限制
- `"Too many sources"` - 批量操作超过数量限制
- `"Too many addresses"` - 批量添加白名单超过数量限制
- `"Array length mismatch"` - 数组长度不匹配
- `"Empty arrays"` - 数组为空
- `"ReentrancyGuardReentrantCall"` - 重入攻击
- `"TokenManager: renounceOwnership is disabled for security"` - 禁用放弃所有权操作
- `"New owner cannot be zero address"` - 新Owner不能为零地址

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

### 角色设置和权限管理
```bash
# 1. 查询当前Owner
cast call --rpc-url $RPC $PROXY_ADDRESS "owner()"

# 2. Owner授予Minter角色
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "grantMinterRole(address)" $MINTER_ADDRESS --legacy

# 3. Owner授予Burner角色  
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "grantBurnerRole(address)" $BURNER_ADDRESS --legacy

# 4. 查询角色数量
cast call --rpc-url $RPC $PROXY_ADDRESS "getMinterRoleCount()"
cast call --rpc-url $RPC $PROXY_ADDRESS "getBurnerRoleCount()"

# 5. 查询所有角色成员
cast call --rpc-url $RPC $PROXY_ADDRESS "getAllMinters()"
cast call --rpc-url $RPC $PROXY_ADDRESS "getAllBurners()"

# 6. 分页查询角色成员
cast call --rpc-url $RPC $PROXY_ADDRESS "getMintersPaginated(uint256,uint256)" 0 10
cast call --rpc-url $RPC $PROXY_ADDRESS "getBurnersPaginated(uint256,uint256)" 0 10

# 7. 检查角色权限
# ADMIN_ROLE = 0x0000000000000000000000000000000000000000000000000000000000000000
cast call --rpc-url $RPC $PROXY_ADDRESS \
  "hasRole(bytes32,address)" 0x0000000000000000000000000000000000000000000000000000000000000000 $ADDRESS

# 8. 撤销角色 (如果需要)
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "revokeMinterRole(address)" $OLD_MINTER_ADDRESS --legacy

# 9. 安全转移所有权 (自动管理ADMIN_ROLE)
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "transferOwnership(address)" $NEW_OWNER_ADDRESS --legacy

# ❌ 禁用操作 - 会失败
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "renounceOwnership()" --legacy
# Error: TokenManager: renounceOwnership is disabled for security
```

### 白名单管理
```bash
# 单个添加白名单
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "addMintWhitelist(address)" $ADDRESS --legacy

cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "addBurnWhitelist(address)" $ADDRESS --legacy

# 批量添加白名单 (最多20个地址)
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "batchAddMintWhitelist(address[])" \
  "[$ADDRESS1,$ADDRESS2,$ADDRESS3]" --legacy

cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "batchAddBurnWhitelist(address[])" \
  "[$ADDRESS1,$ADDRESS2,$ADDRESS3]" --legacy

# 分页查询白名单 (最多返回100个)
cast call --rpc-url $RPC $PROXY_ADDRESS \
  "getMintWhitelist(uint256,uint256)" 0 50

cast call --rpc-url $RPC $PROXY_ADDRESS \
  "getBurnWhitelist(uint256,uint256)" 0 50

# 查询白名单数量
cast call --rpc-url $RPC $PROXY_ADDRESS "getMintWhitelistCount()"
cast call --rpc-url $RPC $PROXY_ADDRESS "getBurnWhitelistCount()"

# 检查地址权限
cast call --rpc-url $RPC $PROXY_ADDRESS \
  "isMintAllowed(address)" $ADDRESS

cast call --rpc-url $RPC $PROXY_ADDRESS \
  "isBurnAllowed(address)" $ADDRESS

# 移除白名单
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "removeMintWhitelist(address)" $ADDRESS --legacy

cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "removeBurnWhitelist(address)" $ADDRESS --legacy
```

### 代币操作 (需要专门角色)
```bash
# 1. 首先设置白名单 (Owner操作)
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "addMintWhitelist(address)" $TO_ADDRESS --legacy

cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "addBurnWhitelist(address)" $FROM_ADDRESS --legacy

# 2. Minter铸造10个代币 (只有Minter可以操作)
cast send --private-key $MINTER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "mint(address,uint256)" $TO_ADDRESS 10000000000000000000 --legacy

# 3. Burner销毁5个代币 (只有Burner可以操作)
cast send --private-key $BURNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "burn(address,uint256)" $FROM_ADDRESS 5000000000000000000 --legacy

# 4. 批量操作 (最多20个地址)
# 批量铸造
cast send --private-key $MINTER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "batchMint(address[],uint256[])" \
  "[$TO_ADDRESS1,$TO_ADDRESS2]" \
  "[1000000000000000000,2000000000000000000]" --legacy

# 批量销毁
cast send --private-key $BURNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "batchBurn(address[],uint256[])" \
  "[$FROM_ADDRESS1,$FROM_ADDRESS2]" \
  "[500000000000000000,1000000000000000000]" --legacy

# ❌ Owner无法直接mint/burn (会失败)
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "mint(address,uint256)" $TO_ADDRESS 1000000000000000000 --legacy
# Error: AccessControlUnauthorizedAccount: account ... is missing role ...
```

### 紧急控制
```bash
# 暂停合约
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "pause()" --legacy

# 恢复合约
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "unpause()" --legacy

# 查询状态
cast call --rpc-url $RPC $PROXY_ADDRESS "paused()"
cast call --rpc-url $RPC $PROXY_ADDRESS "isActive()"
cast call --rpc-url $RPC $PROXY_ADDRESS "isPrecompileAvailable()"
```

### 系统诊断
```bash
# 查询版本和常量
cast call --rpc-url $RPC $PROXY_ADDRESS "VERSION()"
cast call --rpc-url $RPC $PROXY_ADDRESS "MAX_BATCH_SIZE()"
cast call --rpc-url $RPC $PROXY_ADDRESS "MAX_WHITELIST_RETURN()"

# 查询角色标识符
cast call --rpc-url $RPC $PROXY_ADDRESS "ADMIN_ROLE()"
cast call --rpc-url $RPC $PROXY_ADDRESS "MINTER_ROLE()"
cast call --rpc-url $RPC $PROXY_ADDRESS "BURNER_ROLE()"

# 查询激活状态
cast call --rpc-url $RPC $PROXY_ADDRESS "activationBlock()"
cast call --rpc-url $RPC $PROXY_ADDRESS "isActive()"
```

---

## ⚠️ 重要注意事项

1. **角色分离**: 严格的角色分离设计，Owner不自动拥有mint/burn权限
2. **权限管理**: Owner权限非常重要，务必安全保管私钥
3. **激活机制**: 合约必须先激活才能进行mint/burn操作
4. **白名单限制**: mint/burn操作都需要地址在相应白名单中
5. **白名单默认拒绝**: 白名单为空时默认禁止所有操作(安全优先)
6. **余额保护**: 不能销毁地址的全部余额
7. **批量限制**: 批量操作最多20个地址，白名单查询最多返回100个
8. **重入保护**: mint/burn操作都有重入保护
9. **暂停功能**: 可用于紧急情况下停止所有操作
10. **升级能力**: 通过代理合约支持未来升级
11. **所有权安全**: `renounceOwnership()` 已被禁用，防止合约失去管理员
12. **角色枚举**: 支持查询和枚举所有角色成员
13. **安全转移**: 转移Owner时自动管理ADMIN_ROLE

---

## 🔗 相关文档

- [完整设计文档](TokenManager_Complete_Design.md)
- [部署脚本使用说明](../scripts/deploy_tokenmanager.sh)
- [OpenZeppelin Contracts 文档](https://docs.openzeppelin.com/contracts/) 