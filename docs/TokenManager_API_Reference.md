# Token Manager API 接口文档

## 📋 概述

Token Manager V1 是一个可升级的代币管理系统，提供安全的代币铸造(mint)和清理(cleanup)功能。基于 OpenZeppelin 标准合约构建，采用简化的地址控制模式，具有权限控制、暂停机制和升级能力。

**合约地址**: 通过 TokenManagerProxy 代理合约访问  
**Precompile地址**: `0x0000000000000000000000000000000000001001`  
**版本**: v1.0.0

### 🚨 系统限制

| 限制项 | 数值 | 说明 |
|--------|------|------|
| **单一地址设计** | 每个角色只能有1个地址 | Admin/Operator采用单一地址模式，物理上无法设置多个 |
| **Cleanup余额保护** | 必须保留≥1 wei | 防止目标地址余额被完全清零 |
| **权限分离** | Owner/Admin/Operator三级权限 | 两个权限体系分离，但允许同一地址同时担任两个角色 |
| **接口简化** | 移除冗余函数 | 使用Solidity标准getter替代自定义查询函数 |

---

## 🔐 地址权限表

### 权限矩阵

| 功能/接口 | Owner | Admin | Operator |
|-----------|:-----:|:-----:|:--------:|
| **系统控制** ||||
| pause/<br/>unpause | ✅ | ❌ | ❌ |
| transferOwnership | ✅ | ❌ | ❌ |
| setActivationBlock | ✅ | ❌ | ❌ |
| **地址管理** ||||
| setAdmin | ❌ | ✅ | ❌ |
| setOperator | ❌ | ✅ | ❌ |
| **核心操作** ||||
| mint (到操作员) | ❌ | ❌ | ✅ |
| cleanup (清理目标地址) | ❌ | ❌ | ✅ |
| **查询接口** ||||
| admin() / operator() | ✅ | ✅ | ✅ |
| isActive() / VERSION() | ✅ | ✅ | ✅ |

### 详细接口权限分配

| 角色 | 地址控制 | 权限说明 | 专有接口 (仅此角色可调用) |
|------|----------|----------|--------------------------|
| **Owner** | `owner()` | 系统级权限 | **系统控制**:<br/>• `pause()`<br/>• `unpause()`<br/>• `setActivationBlock(uint256)`<br/>• `transferOwnership(address)`<br/>• `renounceOwnership()` *(已禁用)*<br/>• *合约升级权限 (通过ProxyAdmin)* |
| **Admin** | `admin` 状态变量 | 业务管理员 | **地址管理**:<br/>• `setAdmin(address)` *(转移admin权限)*<br/>• `setOperator(address)` *(支持零地址移除)*<br/>*(注：admin权限完全独立于owner)* |
| **Operator** | `operator` 状态变量 | 业务操作员 | **代币操作**:<br/>• `mint(uint256)` *(铸造到操作员地址)*<br/>• `cleanup()` *(清理目标地址)*<br/>*(注：operator可为零地址，表示无操作员)* |

### 公开查询接口 (所有用户可调用)

| 接口类型 | 具体接口 |
|----------|----------|
| **基础信息** | • `owner()` - 获取Owner地址 *(自动生成)*<br/>• `getAdmin()` - 获取Admin地址<br/>• `VERSION()` - 获取版本信息<br/>• `hasRole(bytes32,address)` - 检查角色权限 *(标准接口)* |
| **系统状态** | • `isActive()` - 检查激活状态<br/>• `activationBlock()` - 获取激活区块 *(自动生成)*<br/>• `paused()` - 检查暂停状态 *(自动生成)* |
| **常量查询** | • `ADMIN_ROLE()` - 管理员角色标识符 *(自动生成)*<br/>• `OPERATOR_ROLE()` - 操作员角色标识符 *(自动生成)* |
| **OpenZeppelin标准接口** | • `hasRole(bytes32,address)` - 检查角色权限 *(标准接口)*<br/>• `getRoleMember(bytes32,uint256)` - 获取角色成员 *(标准接口)*<br/>• `getRoleMemberCount(bytes32)` - 获取角色成员数量 *(单一角色设计：OPERATOR应返回0或1)*<br/>• `getRoleMembers(bytes32)` - 获取角色成员列表 *(标准接口，从父合约继承)* |
| **角色查询** | • `getCurrentOperator()` - 获取当前Operator地址 *(单一角色)* |

**重要说明**:
- 合约Owner和Admin完全分离，Owner默认**不拥有任何业务权限**
- Owner只负责系统级操作（合约升级、所有权转移、暂停控制）
- Admin负责业务角色管理（设置/移除操作员）
- Operator负责业务操作执行（铸造代币、清理地址）
- **OPERATOR_ROLE 采用单一角色设计**，每个时刻只能有一个地址持有
- ADMIN_ROLE可以管理OPERATOR_ROLE
- 转移Owner时，Admin权限保持不变
- **角色查询通过 `getCurrentOperator()` 获取单一角色持有者**

---

## 🔧 核心功能接口

### 代币操作

#### `mint(uint256 amount)`
铸造代币到操作员地址

**权限**: 仅Operator角色(onlyRole(OPERATOR_ROLE)) + 重入保护(nonReentrant)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile)

**参数**:
- `amount`: 铸造数量 (Wei, 必须大于0)

**逻辑**: 代币直接铸造到调用者(操作员)的地址

**事件**: `TokenMinted(address indexed operator, uint256 amount)`

**调用示例**:
```bash
cast send --private-key $OPERATOR_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "mint(uint256)" $AMOUNT --legacy
```

#### `cleanup()`
清理目标地址的代币

**权限**: 仅Operator角色(onlyRole(OPERATOR_ROLE)) + 重入保护(nonReentrant)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile)

**目标地址**: `0x000000000000000000000000000000000000dEaD` (固定目标地址)

**逻辑**: 
- 清理目标地址的所有余额，但保留1 wei
- 如果目标地址余额已经是1 wei或更少，操作仍然成功（幂等性）

**余额保护**:
- <span style="color: #fff; background: #d32f2f; font-weight: bold; padding: 2px 4px; border-radius: 2px;">清理后必须保留至少1 wei</span>
- 这确保了目标地址在状态树中的存在，防止地址被删除

**事件**: `TargetAddressCleaned(address indexed operator)`

**调用示例**:
```bash
cast send --private-key $OPERATOR_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "cleanup()" --legacy
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

#### `transferAdminRole(address newAdmin)`
转移Admin权限

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `newAdmin` - 新Admin地址 (不能为零地址，不能是自己，不能已有Admin权限)  
**逻辑**: 
1. 撤销当前Admin的ADMIN_ROLE
2. 授予新Admin的ADMIN_ROLE

**事件**: `AdminRoleTransferred(address indexed oldAdmin, address indexed newAdmin)`

### 角色管理

#### `setOperator(address newOperator)`
设置Operator地址 **(🔄 单一角色设计：类似Admin模式)**

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**参数**: `newOperator` - 新的Operator地址（可以是零地址以清空角色）  
**逻辑**: 单一角色管理，自动替换当前Operator
**事件**: 
- `RoleRevoked(...)` *(如果替换了旧Operator)*
- `RoleGranted(...)` *(如果设置了新Operator)*

#### `removeOperator()`
移除当前Operator **(清空Operator角色)**

**权限**: 仅Admin(onlyRole(ADMIN_ROLE))  
**逻辑**: 清空当前Operator角色，使系统暂时无Operator
**事件**: `RoleRevoked(...)` *(如果有当前Operator)*

#### `hasRole(bytes32 role, address account) → bool`
检查地址是否拥有指定角色 (OpenZeppelin 标准)

**权限**: 公开查询  
**参数**: 
- `role` - 角色标识符 (ADMIN_ROLE/OPERATOR_ROLE)
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

#### `getRoleMembers(bytes32 role) → address[]`
获取指定角色的所有成员 (继承自父合约)

**权限**: 公开查询  
**参数**: `role` - 角色标识符  
**返回**: 角色成员地址数组

**🔍 单一角色设计说明**:
- **OPERATOR_ROLE**: 每个角色最多只有1个成员
- 使用 `getRoleMemberCount(role)` 将返回 0 或 1
- 使用 `getRoleMember(role, 0)` 获取唯一成员（如果存在）
- **推荐使用** `getCurrentOperator()` 进行查询

### 角色查询 (基于 AccessControlEnumerable)

#### `getCurrentOperator() → address`
获取当前Operator地址 **(单一角色设计)**

**权限**: 公开查询  
**返回**: 当前Operator地址，如果无则返回零地址
**说明**: 专为单一角色设计优化的查询函数

---

## 🛑 系统控制

### 暂停机制

#### `pause()`
暂停合约操作

**权限**: 仅Owner(onlyOwner)  
**状态**: 暂停所有mint/cleanup操作  
**事件**: `Paused(address account)`

#### `unpause()`
恢复合约操作

**权限**: 仅Owner(onlyOwner)  
**状态**: 恢复所有mint/cleanup操作  
**事件**: `Unpaused(address account)`

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
**返回**: 版本字符串 ("1.0.0")

---

## 📊 事件列表

### 系统事件
- `Initialized(address indexed owner, address indexed admin, uint256 activationBlock)`
- `ActivationBlockSet(uint256 activationBlock)`
- `AdminRoleTransferred(address indexed oldAdmin, address indexed newAdmin)`

### OpenZeppelin 标准事件
- `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)` *(OwnableUpgradeable)*
- `Paused(address account)` *(PausableUpgradeable)*
- `Unpaused(address account)` *(PausableUpgradeable)*
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)` *(AccessControlEnumerableUpgradeable)*
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)` *(AccessControlEnumerableUpgradeable)*

### 代币操作事件
- `TokenMinted(address indexed operator, uint256 amount)`
- `TargetAddressCleaned(address indexed operator)`

---

## ⚠️ 重要注意事项

### 安全特性
1. **Owner和Admin分离**: Owner只负责系统级操作，Admin负责业务操作
2. **重入保护**: 所有mint/cleanup操作都有重入保护
3. **暂停机制**: 紧急情况下可以暂停所有操作
4. **单一角色设计**: 确保每个业务角色只有一个持有者
5. **余额保护**: cleanup操作保留1 wei防止地址删除

### 使用限制
1. **Precompile依赖**: 需要precompile可用才能进行mint/cleanup操作
2. **激活检查**: 合约必须激活才能进行操作
3. **单一Operator**: 系统同时只能有一个操作员
4. **固定目标地址**: cleanup操作只能清理预定义的目标地址

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
- `getRoleMembers(bytes32)` - 获取角色成员列表 (继承自父合约)

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

---

## 🚀 预编译合约接口

### Token Manager Precompile

**地址**: `0x0000000000000000000000000000000000001001`

#### 操作码

| 操作码 | 值 | 功能 | 权限要求 |
|--------|-----|------|----------|
| `TEST_OP` | `0x01` | 测试连接 | 无 |
| `MINT_OP` | `0x02` | 铸造代币 | 仅合约管理器 |
| `CLEAN_OP` | `0x03` | 清理目标地址 | 仅合约管理器 |

#### 目标地址

- **TARGET_ADDRESS**: `0x000000000000000000000000000000000000dEaD`
- **用途**: cleanup操作的目标地址
- **保护机制**: 清理后始终保留1 wei以维护地址在状态树中的存在

#### 预编译逻辑

1. **MINT_OP**: 向调用者地址铸造指定数量的代币
2. **CLEAN_OP**: 清理目标地址的余额，但保留1 wei
3. **权限控制**: 只有授权的合约管理器地址可以调用铸造和清理操作

**重要说明**: 预编译合约确保了原子性操作和gas效率，同时提供了必要的安全保护机制。

---

## 🔧 测试脚本修复记录

### 已修复的关键问题

#### 1. 私钥地址不匹配问题 ✅
**问题**: 原脚本中 `OPERATOR_PRIVATE_KEY` 对应地址与定义不符
- **错误**: `OPERATOR="0x07d3C7978836067b89ae6Ed0BEaa106ef012e353"`
- **实际**: `OPERATOR_PRIVATE_KEY` 对应 `0xED54a7C1d8634BB589f24Bb7F05a5554b36F9618`
- **修复**: `OPERATOR=$(cast wallet address --private-key "$OPERATOR_PRIVATE_KEY")`

#### 2. 状态重置机制 ✅  
**问题**: 测试前假设合约处于期望状态，导致测试失败
- **修复**: 添加状态检查和重置逻辑
- **实现**: 只有当前状态与期望不符时才进行重置，避免"已是当前operator"错误

#### 3. 转账逻辑错误 ✅
**问题**: cleanup测试中使用operator转账准备测试环境
- **错误**: operator余额不足无法转账2 ETH
- **修复**: 改用admin地址准备测试环境 (余额充足: 99989 ETH)

#### 4. 状态恢复机制 ✅
**问题**: 测试完成后状态被修改，影响下次测试
- **修复**: 添加完整的状态恢复步骤，确保admin/owner/operator都回到原始状态
- **实现**: 使用条件检查避免不必要的设置操作

### 测试脚本最佳实践

#### 状态管理原则
1. **测试前重置**: 自动检查并重置到期望状态
2. **测试后恢复**: 自动恢复所有修改的状态
3. **幂等性设计**: 支持重复运行而不出错
4. **条件检查**: 只有需要时才进行状态修改

#### 地址配置建议  
```bash
# ✅ 正确：从私钥计算地址
OPERATOR=$(cast wallet address --private-key "$OPERATOR_PRIVATE_KEY")

# ❌ 错误：硬编码可能不匹配的地址
OPERATOR="0x07d3C7978836067b89ae6Ed0BEaa106ef012e353"
```

#### 权限分离策略
- **Admin**: 准备测试环境，管理权限 (余额充足)
- **Owner**: 系统级控制，权限转移测试
- **Operator**: 业务操作，执行mint/cleanup

### 当前配置状态

#### 地址配置 (已验证)
- **Admin**: `0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534` (余额: 99989 ETH)  
- **Owner**: `0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15` (余额: ~2 ETH)
- **Operator**: `0xED54a7C1d8634BB589f24Bb7F05a5554b36F9618` (动态计算)

#### 测试流程
1. 🔄 **状态重置** - 检查并重置admin/operator
2. 🧪 **功能测试** - 7个测试步骤完整覆盖  
3. 🔄 **状态恢复** - 恢复所有修改的状态
4. ✅ **幂等验证** - 支持连续多次运行