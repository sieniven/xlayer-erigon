# Token Manager API 接口文档

## 📋 概述

Token Manager V1 是一个可升级的代币管理系统，提供安全的代币跨链(bridgeFrom)和清理(cleanup)功能。基于 OpenZeppelin 标准合约构建，采用简化的地址控制模式，具有权限控制、暂停机制和升级能力。

**合约地址**: 通过 TokenManagerProxy 代理合约访问  
**Precompile地址**: `0x0000000000000000000000000000000000001001`  
**版本**: v1.0.0

### 🚨 系统限制

| 限制项 | 数值 | 说明 |
|--------|------|------|
| **简单地址控制** | 每个权限只能有1个地址 | Admin/Operator采用单一地址变量，非角色系统 |
| **Cleanup余额保护** | 必须保留≥1 wei | 防止目标地址余额被完全清零 |
| **权限分离** | Owner/Admin/Operator三级权限 | 三个独立地址变量，允许同一地址担任多个角色 |
| **接口简化** | 移除冗余函数 | 使用Solidity标准getter替代自定义查询函数 |

---

## 🔐 地址权限表

### 权限矩阵

| 功能/接口                  | Owner | Admin | Operator |
|------------------------|:-----:|:-----:|:--------:|
| **系统控制**               ||||
| pause/<br/>unpause     | ✅ | ❌ | ❌ |
| transferOwnership      | ✅ | ❌ | ❌ |
| setActivationBlock     | ✅ | ❌ | ❌ |
| **地址管理**               ||||
| setAdmin               | ❌ | ✅ | ❌ |
| setOperator            | ❌ | ✅ | ❌ |
| **核心操作**               ||||
| bridgeFrom (跨链到操作员地址)  | ❌ | ❌ | ✅ |
| cleanup (清理目标地址)       | ❌ | ❌ | ✅ |
| **查询接口**               ||||
| admin() / operator()   | ✅ | ✅ | ✅ |
| isActive() / VERSION() | ✅ | ✅ | ✅ |

### 详细接口权限分配

| 权限角色 | 地址查询 | 权限说明 | 专有接口 (仅此权限可调用) |
|------|----------|----------|--------------------------|
| **Owner** | `owner()` | 系统级权限 | **系统控制**:<br/>• `pause()`<br/>• `unpause()`<br/>• `setActivationBlock(uint256)`<br/>• `transferOwnership(address)`<br/>• `renounceOwnership()` *(已禁用)*<br/>• *合约升级权限 (通过ProxyAdmin)* |
| **Admin** | `admin()` | 业务管理权限 | **地址管理**:<br/>• `setAdmin(address)` *(更换admin地址)*<br/>• `setOperator(address)` *(设置/移除operator)*<br/>*(注：admin权限完全独立于owner)* |
| **Operator** | `operator()` | 业务操作权限 | **代币操作**:<br/>• `bridgeFrom(uint256)` *(跨链到操作员地址)*<br/>• `cleanup()` *(清理目标地址)*<br/>*(注：operator可为零地址，表示无操作员)* |

### 公开查询接口 (所有用户可调用)

| 接口类型 | 具体接口 |
|----------|----------|
| **基础信息** | • `owner()` - 获取Owner地址 *(自动生成)*<br/>• `admin()` - 获取Admin地址 *(自动生成)*<br/>• `operator()` - 获取Operator地址 *(自动生成)*<br/>• `VERSION()` - 获取版本信息 |
| **系统状态** | • `isActive()` - 检查激活状态<br/>• `activationBlock()` - 获取激活区块 *(自动生成)*<br/>• `paused()` - 检查暂停状态 *(自动生成)* |
| **常量查询** | • 无角色常量（使用简单地址控制） |
| **OpenZeppelin标准接口** | • `paused()` - 检查暂停状态 *(PausableUpgradeable)*<br/>• `owner()` - 获取合约所有者 *(OwnableUpgradeable)* |
| **地址查询** | • `operator()` - 获取当前Operator地址 *(状态变量)* |

**重要说明**:
- 合约Owner和Admin完全分离，Owner默认**不拥有任何业务权限**
- Owner只负责系统级操作（合约升级、所有权转移、暂停控制）
- Admin负责业务角色管理（设置/移除操作员）
- Operator负责业务操作执行（跨链代币、清理地址）
- **operator 采用单一地址设计**，每个时刻只能有一个地址
- admin可以管理operator地址
- 转移Owner时，admin权限保持不变
- **地址查询通过 `operator()` 获取当前操作员地址**

---

## 🔧 核心功能接口

### 代币操作

#### `bridgeFrom(uint256 amount)`
跨链代币到操作员地址

**权限**: 仅Operator(onlyOperator) + 重入保护(nonReentrant)  
**状态**: 需要激活(onlyActive) + 未暂停(whenNotPaused) + Precompile可用(onlyWithPrecompile)

**参数**:
- `amount`: 跨链数量 (Wei, 必须大于0)

**逻辑**: 代币跨链到调用者(操作员)的地址

**事件**: `TokenBridged(address indexed operator, uint256 amount)`

**调用示例**:
```bash
cast send --private-key $OPERATOR_KEY --rpc-url $RPC $PROXY_ADDRESS \
  "bridgeFrom(uint256)" $AMOUNT --legacy
```

#### `cleanup()`
清理目标地址的代币

**权限**: 仅Operator(onlyOperator) + 重入保护(nonReentrant)  
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
**返回**: 当前Owner地址 (仅系统级权限)

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

#### `admin() → address`
获取管理员地址

**权限**: 公开查询  
**返回**: 当前Admin地址 (自动生成的getter函数)

### Admin权限管理

#### `setAdmin(address newAdmin)`
设置新的Admin地址

**权限**: 仅当前Admin(onlyAdmin)  
**参数**: `newAdmin` - 新Admin地址 (不能为零地址，不能与当前admin相同)  
**逻辑**: 
1. 直接替换admin状态变量
2. 发出AdminChanged事件

**事件**: `AdminChanged(address indexed oldAdmin, address indexed newAdmin)`

### 地址管理

#### `setOperator(address newOperator)`
设置Operator地址

**权限**: 仅Admin(onlyAdmin)  
**参数**: `newOperator` - 新的Operator地址（可以是零地址以清空）  
**逻辑**: 直接替换operator状态变量
**事件**: `OperatorChanged(address indexed oldOperator, address indexed newOperator)`

**注意**: 没有单独的removeOperator函数，使用`setOperator(address(0))`来移除Operator

**注意**: 合约不使用AccessControl角色系统，直接使用`admin()`和`operator()`状态变量查询

**注意**: 合约不继承AccessControlEnumerable，不提供角色枚举功能

### 地址查询

#### `operator() → address`
获取当前Operator地址

**权限**: 公开查询  
**返回**: 当前Operator地址，如果未设置则返回零地址
**说明**: 自动生成的getter函数

---

## 🛑 系统控制

### 暂停机制

#### `pause()`
暂停合约操作

**权限**: 仅Owner(onlyOwner)  
**状态**: 暂停所有bridgeFrom/cleanup操作  
**事件**: `Paused(address account)`

#### `unpause()`
恢复合约操作

**权限**: 仅Owner(onlyOwner)  
**状态**: 恢复所有bridgeFrom/cleanup操作  
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
**逻辑**: 当前区块 >= 激活区块

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
- `AdminChanged(address indexed oldAdmin, address indexed newAdmin)`
- `OperatorChanged(address indexed oldOperator, address indexed newOperator)`

### OpenZeppelin 标准事件
- `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)` *(OwnableUpgradeable)*
- `Paused(address account)` *(PausableUpgradeable)*
- `Unpaused(address account)` *(PausableUpgradeable)*

### 代币操作事件
- `TokenBridged(address indexed operator, uint256 amount)`
- `TargetAddressCleaned(address indexed operator)`

---

## ⚠️ 重要注意事项

### 安全特性
1. **Owner和Admin分离**: Owner只负责系统级操作，Admin负责业务操作
2. **重入保护**: 所有bridgeFrom/cleanup操作都有重入保护
3. **暂停机制**: 紧急情况下可以暂停所有操作
4. **单一地址设计**: 确保每个业务权限只有一个地址持有者
5. **余额保护**: cleanup操作保留1 wei防止地址删除

### 使用限制
1. **Precompile依赖**: 需要precompile可用才能进行bridgeFrom/cleanup操作
2. **激活检查**: 合约必须激活才能进行操作
3. **单一Operator**: 系统同时只能有一个操作员
4. **固定目标地址**: cleanup操作只能清理预定义的目标地址

### 错误处理
- 所有操作都有明确的错误信息
- 权限检查失败会抛出相应的revert错误信息
- 状态检查失败会抛出相应的错误信息

### OpenZeppelin 标准接口说明

Token Manager 合约基于 OpenZeppelin 标准合约构建，继承了以下标准接口：

#### 简单地址控制接口
- `admin()` - 获取admin地址 (状态变量getter)
- `operator()` - 获取operator地址 (状态变量getter)
- `setAdmin(address)` - 设置admin地址 (仅admin可调用)
- `setOperator(address)` - 设置operator地址 (仅admin可调用)

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
- `AdminChanged(address, address)` / `OperatorChanged(address, address)` - 地址变更事件

**重要说明**: 合约基于 OwnableUpgradeable 和 PausableUpgradeable，使用简单地址控制而非 AccessControl 角色系统，提供更轻量级的权限管理。

---

## 🚀 预编译合约接口

### Token Manager Precompile

**地址**: `0x0000000000000000000000000000000000001001`

#### 操作码

| 操作码 | 值 | 功能 | 权限要求 |
|--------|-----|------|----------|
| `TEST_OP` | `0x01` | 测试连接 | 无 |
| `BRIDGE_OP` | `0x02` | 跨链代币 | 仅合约管理器 |
| `CLEAN_OP` | `0x03` | 清理目标地址 | 仅合约管理器 |

#### 目标地址

- **TARGET_ADDRESS**: `0x000000000000000000000000000000000000dEaD`
- **用途**: cleanup操作的目标地址
- **保护机制**: 清理后始终保留1 wei以维护地址在状态树中的存在

#### 预编译逻辑

1. **BRIDGE_OP**: 向调用者地址跨链指定数量的代币
2. **CLEAN_OP**: 清理目标地址的余额，但保留1 wei
3. **权限控制**: 只有授权的合约管理器地址可以调用跨链和清理操作

**重要说明**: 预编译合约确保了原子性操作和gas效率，同时提供了必要的安全保护机制。

