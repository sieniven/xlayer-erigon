# Token Manager API 接口文档

## 📋 概述

Token Manager V1 是一个可升级的代币管理系统，提供安全的代币铸造(mint)和销毁(burn)功能。基于 OpenZeppelin 标准合约构建，具有权限控制、暂停机制和升级能力。

**合约地址**: 通过 TokenManagerProxy 代理合约访问  
**Precompile地址**: `0x0000000000000000000000000000000000008888`  
**版本**: v1.0.0

### 🚨 系统限制

| 限制项 | 数值 | 说明 |
|--------|------|------|
| **Mint白名单最大容量** | 500个地址 | 防止Gas耗尽(OOG)，确保`removeMintWhitelist`操作正常 |
| **分页查询最大返回数** | 100个地址 | 防止单次查询OOG |
| **Burn余额保护** | 必须保留≥1 wei | 防止地址余额被完全清零 |

---

## 🔐 角色权限表

### 权限矩阵

| 功能/接口 | Owner | ADMIN_ROLE | MINTER_ROLE | BURNER_ROLE |
|-----------|:-----:|:----------:|:-----------:|:-----------:|
| **系统控制** |||||
| pause/unpause | ✅ | ❌ | ❌ | ❌ |
| setActivationBlock | ✅ | ❌ | ❌ | ❌ |
| transferOwnership | ✅ | ❌ | ❌ | ❌ |
| **角色管理** |||||
| grantMinterRole/revokeMinterRole | ❌ | ✅ | ❌ | ❌ |
| grantBurnerRole/revokeBurnerRole | ❌ | ✅ | ❌ | ❌ |
| transferAdminRole | ❌ | ✅ | ❌ | ❌ |
| getMintersPaginated/getBurnersPaginated | ❌ | ✅ | ❌ | ❌ |
| **白名单管理** |||||
| addMintWhitelist/removeMintWhitelist | ❌ | ✅ | ❌ | ❌ |
| **核心操作** |||||
| mint | ❌ | ❌ | ✅ | ❌ |
| burn | ❌ | ❌ | ❌ | ✅ |
| **查询接口** |||||
| getMinterRoleCount/getBurnerRoleCount | ✅ | ✅ | ✅ | ✅ |
| getMintWhitelist/getBurnWhitelist | ✅ | ✅ | ✅ | ✅ |
| isMintAllowed/isBurnAllowed | ✅ | ✅ | ✅ | ✅ |
| 基础查询接口（owner/getAdmin/hasAdmin等） | ✅ | ✅ | ✅ | ✅ |

### 详细接口权限分配

| 角色 | 角色标识符 | 权限说明 | 专有接口 (仅此角色可调用) |
|------|------------|----------|--------------------------|
| **Owner** | 合约所有者 | 系统级权限 | **系统控制**:<br/>• `pause()`<br/>• `unpause()`<br/>• `setActivationBlock(uint256)`<br/>• `transferOwnership(address)`<br/>• *合约升级权限 (通过ProxyAdmin)* |
| **ADMIN_ROLE** | `0x0000000000000000000000000000000000000000000000000000000000000000` | 业务管理员 | **角色管理**:<br/>• `grantMinterRole(address)`<br/>• `revokeMinterRole(address)`<br/>• `grantBurnerRole(address)`<br/>• `revokeBurnerRole(address)`<br/>• `getMintersPaginated(uint256,uint256)`<br/>• `getBurnersPaginated(uint256,uint256)`<br/>• `transferAdminRole(address)`<br/><br/>**白名单管理**:<br/>• `addMintWhitelist(address)`<br/>• `removeMintWhitelist(address)`<br/>• *(burn白名单已硬编码，无需管理)* |
| **MINTER_ROLE** | `keccak256("MINTER_ROLE")` | 铸造操作员 | **铸造操作**:<br/>• `mint(address,uint256)` |
| **BURNER_ROLE** | `keccak256("BURNER_ROLE")` | 销毁操作员 | **销毁操作**:<br/>• `burn(address,uint256)` |

### 公开查询接口 (所有用户可调用)

| 接口类型 | 具体接口 |
|----------|----------|
| **基础信息** | • `owner()` - 获取Owner地址 *(自动生成)*<br/>• `getAdmin()` - 获取Admin地址<br/>• `isAdmin(address)` - 检查是否为Admin<br/>• `hasAdmin()` - 检查是否有Admin<br/>• `VERSION()` - 获取版本信息<br/>• `hasRole(bytes32,address)` - 检查角色权限 *(自动生成)* |
| **系统状态** | • `isActive()` - 检查激活状态<br/>• `activationBlock()` - 获取激活区块 *(自动生成)*<br/>• `paused()` - 检查暂停状态 *(自动生成)* |
| **常量查询** | • `MAX_WHITELIST_RETURN()` - 分页查询限制 *(自动生成)*<br/>• `ADMIN_ROLE()` - 管理员角色标识符 *(自动生成)*<br/>• `MINTER_ROLE()` - 铸造者角色标识符 *(自动生成)*<br/>• `BURNER_ROLE()` - 销毁者角色标识符 *(自动生成)* |
| **OpenZeppelin标准接口** | • `hasRole(bytes32,address)` - 检查角色权限 *(自动生成)*<br/>• `getRoleMember(bytes32,uint256)` - 获取角色成员 *(自动生成)*<br/>• `getRoleMemberCount(bytes32)` - 获取角色成员数量 *(自动生成)*<br/>• `getRoleMembersPaginated(bytes32,uint256,uint256)` - 分页获取角色成员 *(自动生成)* |
| **角色查询** | • `getMinterRoleCount()` - 获取Minter角色数量<br/>• `getBurnerRoleCount()` - 获取Burner角色数量 |
| **白名单查询** | • `getMintWhitelist(uint256,uint256)` - 分页获取mint白名单<br/>• `getMintWhitelistCount()` - 获取mint白名单数量<br/>• `mintWhitelist(address)` - 检查mint白名单状态 *(自动生成)*<br/>• `isMintAllowed(address)` - 检查mint权限<br/>• `getBurnWhitelist(uint256,uint256)` - 分页获取burn白名单<br/>• `getBurnWhitelistCount()` - 获取burn白名单数量<br/>• `burnWhitelist(address)` - 检查burn白名单状态 *(自动生成)*<br/>• `isBurnAllowed(address)` - 检查burn权限 |

**重要说明**:
- 合约Owner和Admin完全分离，Owner默认**不拥有任何业务权限**
- Owner只负责系统级操作（合约升级、所有权转移）
- Admin负责所有业务操作（角色管理、白名单管理）
- 要进行mint/burn操作，必须明确授予相应角色
- 每个角色可以授予给多个地址
- ADMIN_ROLE可以管理其他所有角色
- 转移Owner时，Admin权限保持不变
- 角色数量查询(`getMinterRoleCount`, `getBurnerRoleCount`)为公开接口，所有用户可调用

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
- `from`: 销毁代币的地址 (必须在burn白名单中)
- `amount`: 销毁数量 (Wei, 必须大于0，不能销毁全部余额)

**限制**:
- 地址必须在burn白名单中
- <span style="color: #fff; background: #d32f2f; font-weight: bold; padding: 2px 4px; border-radius: 2px;">不能销毁地址的全部余额（必须保留至少1 wei）</span>

**错误处理**:
- `"Insufficient balance for burn"` - 余额不足
- `"Cannot burn entire balance, must leave at least 1 wei"` - 尝试销毁全部余额

**事件**: `TokenBurned(address indexed from, uint256 amount, address indexed burner)`

**调用示例**:
```bash
cast send --private-key $BURNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "burn(address,uint256)" $FROM_ADDRESS $AMOUNT --legacy
```

---

## 🔐 权限管理

### 管理员操作

#### `owner() → address`
获取当前合约Owner地址

**权限**: 公开查询  
**返回**: 当前Owner地址 (仅系统级权限，不拥有ADMIN_ROLE)

#### `transferOwnership(address newOwner)`
转移Owner权限 (安全转移)

**权限**: 仅Owner(onlyOwner)  
**参数**: `newOwner` - 新Owner地址 (不能为零地址)  
**逻辑**: 
1. 转移所有权到新Owner
2. Admin权限保持不变（Owner和Admin完全分离）

**事件**: `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)`

#### `renounceOwnership()`
放弃Owner权限 (已禁用)

**权限**: 已禁用  
**状态**: 调用会直接失败  
**错误**: `"TokenManager: renounceOwnership is disabled for security"`  
**说明**: 为防止合约变成不可管理状态，此功能已被禁用

#### `getAdmin() → address`
获取管理员地址

**权限**: 公开查询  
**返回**: 当前Admin地址 (拥有ADMIN_ROLE的地址)

### Admin权限管理

#### `isAdmin(address account) → bool`
检查地址是否为Admin

**权限**: 公开查询  
**参数**: `account` - 要检查的地址  
**返回**: true(是Admin) / false(不是Admin)

#### `hasAdmin() → bool`
检查是否有Admin

**权限**: 公开查询  
**返回**: true(有Admin) / false(没有Admin)

#### `transferAdminRole(address newAdmin)`
转移Admin权限

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `newAdmin` - 新Admin地址 (不能为零地址，不能是自己，不能已有Admin权限)  
**逻辑**: 
1. 撤销当前Admin的ADMIN_ROLE
2. 授予新Admin的ADMIN_ROLE

**事件**: `AdminRoleTransferred(address indexed oldAdmin, address indexed newAdmin)`

### 角色管理

#### `grantMinterRole(address account)`
授予Minter角色

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要授予角色的地址  
**逻辑**: 如果地址还没有该角色，则授予
**事件**: `MinterRoleGranted(address indexed account, address indexed sender)`

#### `revokeMinterRole(address account)`
撤销Minter角色

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要撤销角色的地址  
**逻辑**: 如果地址拥有该角色，则撤销
**事件**: `MinterRoleRevoked(address indexed account, address indexed sender)`

#### `grantBurnerRole(address account)`
授予Burner角色

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要授予角色的地址  
**逻辑**: 如果地址还没有该角色，则授予
**事件**: `BurnerRoleGranted(address indexed account, address indexed sender)`

#### `revokeBurnerRole(address account)`
撤销Burner角色

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要撤销角色的地址  
**逻辑**: 如果地址拥有该角色，则撤销
**事件**: `BurnerRoleRevoked(address indexed account, address indexed sender)`

#### `hasRole(bytes32 role, address account) → bool`
检查地址是否拥有指定角色 (OpenZeppelin 标准)

**权限**: 公开查询  
**参数**: 
- `role` - 角色标识符 (ADMIN_ROLE/MINTER_ROLE/BURNER_ROLE)
- `account` - 要检查的地址  
**返回**: true(拥有角色) / false(没有角色)

### OpenZeppelin 标准接口 (AccessControlEnumerableUpgradeable)

#### `getRoleMember(bytes32 role, uint256 index) → address`
获取指定角色的成员地址 (OpenZeppelin 标准)

**权限**: 公开查询  
**参数**: 
- `role` - 角色标识符
- `index` - 成员索引 (从0开始)
**返回**: 角色成员地址

#### `getRoleMemberCount(bytes32 role) → uint256`
获取指定角色的成员数量 (OpenZeppelin 标准)

**权限**: 公开查询  
**参数**: `role` - 角色标识符  
**返回**: 角色成员数量

#### `getRoleMembersPaginated(bytes32 role, uint256 offset, uint256 limit) → address[]`
分页获取指定角色的成员列表 (OpenZeppelin 标准)

**权限**: 公开查询  
**参数**: 
- `role` - 角色标识符
- `offset` - 起始索引
- `limit` - 返回数量限制
**返回**: 角色成员地址数组

### 角色查询 (基于 AccessControlEnumerable)

#### `getMinterRoleCount() → uint256`
获取拥有Minter角色的地址数量

**权限**: 公开查询  
**返回**: Minter角色成员数量

#### `getBurnerRoleCount() → uint256`
获取拥有Burner角色的地址数量

**权限**: 公开查询  
**返回**: Burner角色成员数量

#### `getMintersPaginated(uint256 offset, uint256 limit) → address[]`
分页获取Minter角色成员列表

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量限制 (最大100)
**返回**: Minter角色成员地址数组

#### `getBurnersPaginated(uint256 offset, uint256 limit) → address[]`
分页获取Burner角色成员列表

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量限制 (最大100)
**返回**: Burner角色成员地址数组

---

## 📋 白名单管理

### Mint白名单管理 (动态)

#### `addMintWhitelist(address account)`
添加地址到Mint白名单

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要添加的地址  
**限制**: 
- 地址不能已在白名单中
- **🚨 白名单最大容量限制: 500个地址** (防止OOG)  
**事件**: `MintWhitelistAdded(address indexed account, address indexed sender)`

**错误信息**:
- `"Address is already in mint whitelist"` - 地址已存在
- `"Whitelist size limit reached"` - 已达到500个地址上限

#### `removeMintWhitelist(address account)`
从Mint白名单移除地址

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要移除的地址  
**限制**: 地址必须在白名单中  
**逻辑**: 先验证数组操作成功，再更新映射状态
**事件**: `MintWhitelistRemoved(address indexed account, address indexed sender)`

#### `getMintWhitelist(uint256 offset, uint256 limit) → (address[], uint256)`
分页获取Mint白名单

**权限**: 公开查询  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量限制 (最大100)
**返回**: 
- `addresses` - 白名单地址数组
- `total` - 总数量

#### `getMintWhitelistCount() → uint256`
获取Mint白名单总数量

**权限**: 公开查询  
**返回**: 白名单地址总数

#### `mintWhitelist(address account) → bool`
检查地址是否在Mint白名单中

**权限**: 公开查询  
**参数**: `account` - 要检查的地址  
**返回**: true(在白名单中) / false(不在白名单中)

#### `isMintAllowed(address account) → bool`
检查地址是否有Mint权限

**权限**: 公开查询  
**参数**: `account` - 要检查的地址  
**返回**: true(有权限) / false(无权限)  
**逻辑**: 如果白名单为空则拒绝所有，否则检查地址是否在白名单中

### Burn白名单管理 (硬编码)

#### `getBurnWhitelist(uint256 offset, uint256 limit) → (address[], uint256)`
分页获取Burn白名单

**权限**: 公开查询  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量限制 (最大100)
**返回**: 
- `addresses` - 白名单地址数组
- `total` - 总数量

**硬编码地址**:
- `0x000000000000000000000000000000000000dEaD` (Burn地址)
- `0x0000000000000000000000000000000000000000` (零地址)

#### `getBurnWhitelistCount() → uint256`
获取Burn白名单总数量

**权限**: 公开查询  
**返回**: 白名单地址总数 (固定为2)

#### `burnWhitelist(address account) → bool`
检查地址是否在Burn白名单中

**权限**: 公开查询  
**参数**: `account` - 要检查的地址  
**返回**: true(在白名单中) / false(不在白名单中)

#### `isBurnAllowed(address account) → bool`
检查地址是否有Burn权限

**权限**: 公开查询  
**参数**: `account` - 要检查的地址  
**返回**: true(有权限) / false(无权限)  
**逻辑**: 检查地址是否在硬编码的burn白名单中

---

## 🛑 系统控制

### 暂停机制

#### `pause()`
暂停合约操作

**权限**: 仅Owner(onlyOwner)  
**状态**: 暂停所有mint/burn操作  
**事件**: `ContractPaused(address indexed account)`

#### `unpause()`
恢复合约操作

**权限**: 仅Owner(onlyOwner)  
**状态**: 恢复所有mint/burn操作  
**事件**: `ContractUnpaused(address indexed account)`

#### `paused() → bool`
检查合约是否暂停

**权限**: 公开查询  
**返回**: true(已暂停) / false(运行中)

### 激活控制

#### `setActivationBlock(uint256 _activationBlock)`
设置激活区块

**权限**: 仅Owner(onlyOwner)  
**参数**: `_activationBlock` - 激活区块号  
**事件**: `ActivationBlockSet(uint256 activationBlock)`

#### `isActive() → bool`
检查合约是否激活

**权限**: 公开查询  
**返回**: true(已激活) / false(未激活)  
**逻辑**: 当前区块 >= 激活区块 且 Owner不为零地址

#### `activationBlock() → uint256`
获取激活区块号

**权限**: 公开查询  
**返回**: 激活区块号

---

## 🔧 工具函数

#### `VERSION() → string`
获取合约版本

**权限**: 公开查询  
**返回**: 版本字符串 ("v1.0.0")

#### `MAX_WHITELIST_RETURN() → uint256`
获取分页查询限制

**权限**: 公开查询  
**返回**: 最大返回数量 (100)

---

## 📊 事件列表

### 系统事件
- `Initialized(address indexed owner, address indexed admin, uint256 activationBlock)`
- `ActivationBlockSet(uint256 activationBlock)`
- `ContractPaused(address indexed account)`
- `ContractUnpaused(address indexed account)`
- `AdminRoleTransferred(address indexed oldAdmin, address indexed newAdmin)`

### OpenZeppelin 标准事件
- `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)` *(OwnableUpgradeable)*
- `Paused(address account)` *(PausableUpgradeable)*
- `Unpaused(address account)` *(PausableUpgradeable)*
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)` *(AccessControlEnumerableUpgradeable)*
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)` *(AccessControlEnumerableUpgradeable)*

### 角色管理事件
- `MinterRoleGranted(address indexed account, address indexed sender)`
- `MinterRoleRevoked(address indexed account, address indexed sender)`
- `BurnerRoleGranted(address indexed account, address indexed sender)`
- `BurnerRoleRevoked(address indexed account, address indexed sender)`

### 白名单管理事件
- `MintWhitelistAdded(address indexed account, address indexed sender)`
- `MintWhitelistRemoved(address indexed account, address indexed sender)`

### 代币操作事件
- `TokenMinted(address indexed to, uint256 amount, address indexed minter)`
- `TokenBurned(address indexed from, uint256 amount, address indexed burner)`

---

## ⚠️ 重要注意事项

### 安全特性
1. **Owner和Admin分离**: Owner只负责系统级操作，Admin负责业务操作
2. **重入保护**: 所有mint/burn操作都有重入保护
3. **暂停机制**: 紧急情况下可以暂停所有操作
4. **白名单控制**: 严格的mint/burn白名单控制
5. **余额保护**: burn操作不能销毁全部余额

### 使用限制
1. **Precompile依赖**: 需要precompile可用才能进行mint/burn操作
2. **激活检查**: 合约必须激活才能进行操作
3. **分页限制**: 查询接口有分页限制，防止OOG
4. **硬编码burn白名单**: burn白名单不可修改，确保安全性

### 错误处理
- 所有操作都有明确的错误信息
- 权限检查失败会抛出AccessControlUnauthorizedAccount错误
- 状态检查失败会抛出相应的错误信息

### OpenZeppelin 标准接口说明

Token Manager 合约基于 OpenZeppelin 标准合约构建，继承了以下标准接口：

#### AccessControlEnumerableUpgradeable 标准接口
- `hasRole(bytes32, address)` - 检查角色权限
- `getRoleMember(bytes32, uint256)` - 获取角色成员
- `getRoleMemberCount(bytes32)` - 获取角色成员数量
- `getRoleMembersPaginated(bytes32, uint256, uint256)` - 分页获取角色成员

#### OwnableUpgradeable 标准接口
- `owner()` - 获取合约所有者
- `transferOwnership(address)` - 转移所有权
- `renounceOwnership()` - 放弃所有权 (已禁用)

#### PausableUpgradeable 标准接口
- `paused()` - 检查暂停状态
- `pause()` - 暂停合约
- `unpause()` - 恢复合约

#### 标准事件
- `OwnershipTransferred(address, address)` - 所有权转移事件
- `Paused(address)` / `Unpaused(address)` - 暂停/恢复事件
- `RoleGranted(bytes32, address, address)` / `RoleRevoked(bytes32, address, address)` - 角色授予/撤销事件

**重要说明**: 这些标准接口提供了与 OpenZeppelin 生态系统的完全兼容性，开发者可以使用标准的 OpenZeppelin 工具和库来与合约交互。 