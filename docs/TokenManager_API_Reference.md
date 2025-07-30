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
| **Owner** | 合约所有者 | 系统级权限 | **系统控制**:<br/>• `pause()`<br/>• `unpause()`<br/>• `setActivationBlock(uint256)`<br/>• `transferOwnership(address)`<br/>• *合约升级权限 (通过ProxyAdmin)* |
| **ADMIN_ROLE** | `0x0000000000000000000000000000000000000000000000000000000000000000` | 业务管理员 | **角色管理**:<br/>• `grantMinterRole(address)`<br/>• `revokeMinterRole(address)`<br/>• `grantBurnerRole(address)`<br/>• `revokeBurnerRole(address)`<br/>• `getMinterRoleCount()`<br/>• `getBurnerRoleCount()`<br/>• `getMintersPaginated(uint256,uint256)`<br/>• `getBurnersPaginated(uint256,uint256)`<br/>• `transferAdminRole(address)`<br/><br/>**白名单管理**:<br/>• `addMintWhitelist(address)`<br/>• `removeMintWhitelist(address)`<br/>• *(burn白名单已硬编码，无需管理)* |
| **MINTER_ROLE** | `keccak256("MINTER_ROLE")` | 铸造操作员 | **铸造操作**:<br/>• `mint(address,uint256)` |
| **BURNER_ROLE** | `keccak256("BURNER_ROLE")` | 销毁操作员 | **销毁操作**:<br/>• `burn(address,uint256)` |

### 公开查询接口 (所有用户可调用)

| 接口类型 | 具体接口 |
|----------|----------|
| **基础信息** | • `owner()` - 获取Owner地址 *(自动生成)*<br/>• `getAdmin()` - 获取Admin地址<br/>• `isAdmin(address)` - 检查是否为Admin<br/>• `hasAdmin()` - 检查是否有Admin<br/>• `VERSION()` - 获取版本信息<br/>• `hasRole(bytes32,address)` - 检查角色权限 *(自动生成)* |
| **系统状态** | • `isActive()` - 检查激活状态<br/>• `activationBlock()` - 获取激活区块 *(自动生成)*<br/>• `paused()` - 检查暂停状态 *(自动生成)* |
| **常量查询** | • `MAX_WHITELIST_RETURN()` - 分页查询限制 *(自动生成)*<br/>• `ADMIN_ROLE()` - 管理员角色标识符 *(自动生成)*<br/>• `MINTER_ROLE()` - 铸造者角色标识符 *(自动生成)*<br/>• `BURNER_ROLE()` - 销毁者角色标识符 *(自动生成)* |
| **白名单查询** | • `getMintWhitelist(uint256,uint256)` - 分页获取mint白名单<br/>• `getMintWhitelistCount()` - 获取mint白名单数量<br/>• `mintWhitelist(address)` - 检查mint白名单状态 *(自动生成)*<br/>• `isMintAllowed(address)` - 检查mint权限<br/>• `getBurnWhitelist(uint256,uint256)` - 分页获取burn白名单<br/>• `getBurnWhitelistCount()` - 获取burn白名单数量<br/>• `burnWhitelist(address)` - 检查burn白名单状态 *(自动生成)*<br/>• `isBurnAllowed(address)` - 检查burn权限 |

**重要说明**:
- 合约Owner和Admin完全分离，Owner默认**不拥有任何业务权限**
- Owner只负责系统级操作（合约升级、所有权转移）
- Admin负责所有业务操作（角色管理、白名单管理）
- 要进行mint/burn操作，必须明确授予相应角色
- 每个角色可以授予给多个地址
- ADMIN_ROLE可以管理其他所有角色
- 转移Owner时，Admin权限保持不变

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
**事件**: `MinterRoleGranted(address indexed account, address indexed sender)`

#### `revokeMinterRole(address account)`
撤销Minter角色

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要撤销角色的地址  
**事件**: `MinterRoleRevoked(address indexed account, address indexed sender)`

#### `grantBurnerRole(address account)`
授予Burner角色

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要授予角色的地址  
**事件**: `BurnerRoleGranted(address indexed account, address indexed sender)`

#### `revokeBurnerRole(address account)`
撤销Burner角色

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
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

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**返回**: Minter角色成员数量

#### `getBurnerRoleCount() → uint256`
获取拥有Burner角色的地址数量

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**返回**: Burner角色成员数量



#### `getMintersPaginated(uint256 offset, uint256 limit) → address[]`
分页获取Minter角色地址

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量 (自动限制为MAX_WHITELIST_RETURN=100)  
**返回**: 分页的Minter地址数组

#### `getBurnersPaginated(uint256 offset, uint256 limit) → address[]`
分页获取Burner角色地址

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: 
- `offset` - 起始索引
- `limit` - 返回数量 (自动限制为MAX_WHITELIST_RETURN=100)  
**返回**: 分页的Burner地址数组

---

## ⚙️ 系统控制

### 激活控制

#### `setActivationBlock(uint256 _activationBlock)`
设置激活区块

**权限**: 仅Owner(onlyOwner)  
**参数**: `_activationBlock` - 激活区块号  
**事件**: `ActivationBlockSet(uint256 activationBlock)`

#### `isActive() → bool`
检查合约是否已激活

**权限**: 公开查询  
**返回**: true(已激活) / false(未激活)  
**逻辑**: `block.number >= activationBlock && owner() != address(0)`

#### `activationBlock() → uint256`
获取激活区块号

**权限**: 公开查询  
**返回**: 激活区块号

### 暂停控制

#### `pause()`
暂停合约 (紧急停止)

**权限**: 仅Owner(onlyOwner)  
**事件**: `ContractPaused(address indexed sender)`

#### `unpause()`
恢复合约

**权限**: 仅Owner(onlyOwner)  
**事件**: `ContractUnpaused(address indexed sender)`

#### `paused() → bool`
检查合约是否暂停

**权限**: 公开查询  
**返回**: true(已暂停) / false(正常运行)

---

## 📋 白名单管理

### Mint白名单

#### `addMintWhitelist(address account)`
添加地址到mint白名单

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要添加的地址  
**事件**: `MintWhitelistAdded(address indexed account, address indexed sender)`

#### `removeMintWhitelist(address account)`
从mint白名单移除地址

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `account` - 要移除的地址  
**事件**: `MintWhitelistRemoved(address indexed account, address indexed sender)`

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

### Burn白名单 (硬编码，无需管理)

**重要说明**: Burn白名单已硬编码到合约中，无法动态修改。这提高了安全性，防止误操作。

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



---

## 🔍 系统诊断

#### `VERSION() → string`
获取合约版本

**权限**: 公开查询  
**返回**: 版本字符串 "v1.0.0"

#### 常量查询

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
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "setActivationBlock(uint256)" 0 --legacy
```

### 角色设置和权限管理
```bash
# 1. 查询当前Owner
cast call --rpc-url $RPC $PROXY_ADDRESS "owner()"

# 2. Admin授予Minter角色
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "grantMinterRole(address)" $MINTER_ADDRESS --legacy

# 3. Admin授予Burner角色  
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "grantBurnerRole(address)" $BURNER_ADDRESS --legacy

# 4. 查询角色数量
cast call --rpc-url $RPC $PROXY_ADDRESS "getMinterRoleCount()"
cast call --rpc-url $RPC $PROXY_ADDRESS "getBurnerRoleCount()"

# 5. 分页查询角色成员
cast call --rpc-url $RPC $PROXY_ADDRESS "getMintersPaginated(uint256,uint256)" 0 10
cast call --rpc-url $RPC $PROXY_ADDRESS "getBurnersPaginated(uint256,uint256)" 0 10

# 7. 检查角色权限
# ADMIN_ROLE = 0x0000000000000000000000000000000000000000000000000000000000000000
cast call --rpc-url $RPC $PROXY_ADDRESS \
  "hasRole(bytes32,address)" 0x0000000000000000000000000000000000000000000000000000000000000000 $ADDRESS

# 8. 撤销角色 (如果需要)
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "revokeMinterRole(address)" $OLD_MINTER_ADDRESS --legacy

# 9. 转移Admin权限 (如果需要)
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "transferAdminRole(address)" $NEW_ADMIN_ADDRESS --legacy

# 10. 安全转移所有权 (Owner和Admin分离)
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "transferOwnership(address)" $NEW_OWNER_ADDRESS --legacy

# ❌ 禁用操作 - 会失败
cast send --private-key $OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "renounceOwnership()" --legacy
# Error: TokenManager: renounceOwnership is disabled for security
```

### 白名单管理
```bash
# 单个添加mint白名单
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "addMintWhitelist(address)" $ADDRESS --legacy

# 注意: burn白名单已硬编码，无法动态修改

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

# 移除mint白名单
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "removeMintWhitelist(address)" $ADDRESS --legacy

# 注意: burn白名单已硬编码，无法移除
```

### 代币操作 (需要专门角色)
```bash
# 1. 首先设置mint白名单 (Admin操作)
cast send --private-key $ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "addMintWhitelist(address)" $TO_ADDRESS --legacy

# 注意: burn白名单已硬编码，检查是否允许:
cast call --rpc-url $RPC $PROXY_ADDRESS "isBurnAllowed(address)" $FROM_ADDRESS

# 2. Minter铸造10个代币 (只有Minter可以操作)
cast send --private-key $MINTER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "mint(address,uint256)" $TO_ADDRESS 10000000000000000000 --legacy

# 3. Burner销毁5个代币 (只有Burner可以操作)
cast send --private-key $BURNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "burn(address,uint256)" $FROM_ADDRESS 5000000000000000000 --legacy



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
```

### 权限转移 (重要操作)
```bash
# Admin转移权限 (当前Admin执行)
cast send --private-key $CURRENT_ADMIN_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "transferAdminRole(address)" $NEW_ADMIN_ADDRESS --legacy

# Owner转移权限 (当前Owner执行)
cast send --private-key $CURRENT_OWNER_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "transferOwnership(address)" $NEW_OWNER_ADDRESS --legacy

# 验证权限转移结果
cast call --rpc-url $RPC $PROXY_ADDRESS "owner()"
cast call --rpc-url $RPC $PROXY_ADDRESS "getAdmin()"
cast call --rpc-url $RPC $PROXY_ADDRESS "isAdmin(address)" $NEW_ADMIN_ADDRESS
```

### 系统诊断
```bash
# 查询权限信息
cast call --rpc-url $RPC $PROXY_ADDRESS "owner()"
cast call --rpc-url $RPC $PROXY_ADDRESS "getAdmin()"
cast call --rpc-url $RPC $PROXY_ADDRESS "hasAdmin()"

# 查询版本和常量
cast call --rpc-url $RPC $PROXY_ADDRESS "VERSION()"
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