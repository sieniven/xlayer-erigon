# Token Manager 完整设计文档

## 版本信息
- **当前版本**: v1.4.0
- **最后更新**: 2024年12月
- **架构**: Precompile + Smart Contract + Proxy

## 系统概述

Token Manager 是一个基于 Erigon 的双层架构代币管理系统，结合了高性能的 Precompile 合约和灵活的 Solidity 智能合约，提供安全、可升级的代币铸造和销毁功能。

### 核心特性

- ✅ **高性能操作**: 使用 Precompile (0x8888) 实现原子级 mint/burn 操作
- ✅ **OpenZeppelin 安全标准**: 基于经过审计的 OwnableUpgradeable、PausableUpgradeable 等标准合约
- ✅ **可升级设计**: 使用 TransparentUpgradeableProxy 支持合约升级
- ✅ **权限控制**: 多层权限验证，支持所有者管理和紧急暂停
- ✅ **白名单管理**: 灵活的销毁地址白名单系统
- ✅ **可用性检测**: 内置 TEST_OP 用于检测 Precompile 可用性
- ✅ **标准部署**: 使用 CREATE 操作码进行可靠部署

### 系统组件

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   用户/DApp     │───▶│   TokenManager   │───▶│   Precompile    │
│                 │    │   Proxy (0x...)  │    │   (0x8888)      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │ TokenManagerV1   │
                       │ (Implementation) │
                       └──────────────────┘
```

## 架构设计

### 三层架构

#### 1. Precompile 层 (0x8888)
- **职责**: 执行原子级 mint/burn 操作
- **特性**: 高性能、低gas消耗、原生EVM支持
- **权限**: 仅接受来自指定合约管理器的调用
- **操作码**:
  - `0x01`: TEST_OP - 测试可用性（无需权限）
  - `0x02`: MINT_OP - 铸造代币
  - `0x03`: BURN_OP - 销毁代币

#### 2. Smart Contract 层
- **TokenManagerProxy**: 基于 OpenZeppelin TransparentUpgradeableProxy
- **TokenManagerV1**: 业务逻辑实现，继承 OpenZeppelin 标准合约
- **职责**: 权限控制、业务逻辑、白名单管理、事件发射

#### 3. 用户交互层
- **接口**: 标准的 Solidity 合约调用
- **工具**: cast 命令行工具
- **权限**: 基于 onlyOwner 等修饰符的访问控制

### 数据流

```
用户调用 → Proxy合约 → Implementation合约 → Precompile → 状态更新
   ↓              ↓               ↓             ↓
权限检查 → 业务逻辑验证 → 白名单检查 → 原子操作 → 事件发射
```

## Precompile 实现

### 地址与接口
- **地址**: `0x0000000000000000000000000000000000008888`
- **类型**: `mintBurnPrecompile`
- **配置**: `CONFIG_CONTRACT_MANAGER_ADDRESS = 0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab`

### 操作码定义

```go
const (
    TEST_OP = 0x01 // 测试 Precompile 可用性（无需权限）
    MINT_OP = 0x02 // 铸造代币（需要权限验证）
    BURN_OP = 0x03 // 销毁代币（需要权限验证）
)
```

### 核心方法

#### TEST_OP (0x01)
```go
// 输入: [0x01]
// 输出: "OK" (无需权限)
// 用途: 检测 Precompile 可用性
```

#### MINT_OP (0x02)
```go
// 输入: [0x02][32字节地址][32字节数量]
// 输出: 成功时返回空字节
// 权限: 仅 CONFIG_CONTRACT_MANAGER_ADDRESS
```

#### BURN_OP (0x03)
```go
// 输入: [0x03][32字节地址][32字节数量]
// 输出: 成功时返回空字节
// 权限: 仅 CONFIG_CONTRACT_MANAGER_ADDRESS
```

### 安全机制

1. **调用者验证**: 检查 `caller.Address()` 是否为授权合约
2. **输入验证**: 严格的数据长度和格式检查
3. **操作码验证**: 支持的操作码白名单检查
4. **错误处理**: 详细的错误信息返回

## Smart Contract 设计

### TokenManagerProxy (代理合约)

基于 OpenZeppelin `TransparentUpgradeableProxy`：

```solidity
contract TokenManagerProxy is TransparentUpgradeableProxy {
    constructor(
        address logic,
        address admin,
        bytes memory data
    ) TransparentUpgradeableProxy(logic, admin, data) {
        // OpenZeppelin 处理所有代理逻辑
    }
}
```

### TokenManagerV1 (实现合约)

继承关系：
```solidity
contract TokenManagerV1 is 
    Initializable, 
    OwnableUpgradeable, 
    PausableUpgradeable
```

#### 核心状态变量

```solidity
// Precompile 地址
address constant PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000008888;

// 操作码常量
bytes1 constant TEST_OP = 0x01;
bytes1 constant MINT_OP = 0x02;
bytes1 constant BURN_OP = 0x03;

// 状态变量
uint256 public activationBlock;
mapping(address => bool) private burnWhitelist;
address[] private burnWhitelistArray;
```

#### 权限控制

1. **onlyOwner**: 继承自 OwnableUpgradeable
2. **onlyActive**: 检查激活状态
3. **whenNotPaused**: 继承自 PausableUpgradeable
4. **onlyWithPrecompile**: 检查 Precompile 可用性

#### 核心业务功能

##### 铸造功能
```solidity
function mint(address to, uint256 amount) 
    external 
    onlyOwner 
    onlyActive 
    whenNotPaused 
    onlyWithPrecompile
```

##### 销毁功能
```solidity
function burn(address from, uint256 amount) 
    external 
    onlyOwner 
    onlyActive 
    whenNotPaused 
    onlyWithPrecompile
```

##### 白名单管理
```solidity
function addBurnWhitelist(address account) external onlyOwner
function removeBurnWhitelist(address account) external onlyOwner
function getBurnWhitelist() external view returns (address[] memory)
function isBurnAllowed(address account) public view returns (bool)
```

#### 重要设计决策

1. **允许 null 地址**: `addBurnWhitelist` 允许添加 `address(0)`
2. **余额保护**: 在合约层检查"不能销毁全部余额"
3. **统一白名单检查**: 移除重复的 `isBurnWhitelisted`，统一使用 `isBurnAllowed`
4. **Precompile 可用性检查**: 使用 TEST_OP 进行可靠检测

### Precompile 可用性检测

```solidity
function isPrecompileAvailable() public view returns (bool) {
    bytes memory testData = abi.encodePacked(TEST_OP);
    (bool success, bytes memory returnData) = PRECOMPILE_ADDRESS.staticcall(testData);
    
    // 检查是否成功返回 "OK"
    return success && returnData.length == 2 && 
           returnData[0] == 0x4F && returnData[1] == 0x4B; // "OK" in hex
}
```

## 部署计划

### 标准 CREATE 部署

```bash
# 1. 编译合约
solc --bin --evm-version paris TokenManagerV1.sol -o . --overwrite --base-path . --include-path node_modules/
solc --bin --evm-version paris TokenManagerProxy.sol -o . --overwrite --base-path . --include-path node_modules/

# 2. 部署实现合约
cast send --private-key $PRIVATE_KEY --rpc-url $RPC_URL --legacy --create $IMPL_BYTECODE

# 3. 部署代理合约
cast send --private-key $PRIVATE_KEY --rpc-url $RPC_URL --legacy --create "$PROXY_DEPLOY_DATA"

# 4. 初始化合约
cast send --private-key $PRIVATE_KEY --rpc-url $RPC_URL --to $PROXY_ADDRESS $INIT_DATA
```

### 部署脚本

使用 `scripts/deploy_tokenmanager.sh`：

```bash
# 设置环境变量
export PRIVATE_KEY="0x..."
export RPC_URL="http://localhost:8123"
export ACTIVATION_BLOCK="0"  # 立即激活

# 执行部署
./scripts/deploy_tokenmanager.sh
```

## 操作指南

### 管理员操作

#### 基础查询
```bash
# 查询当前所有者
cast call --rpc-url $RPC_URL $PROXY_ADDRESS "owner()"

# 查询激活状态
cast call --rpc-url $RPC_URL $PROXY_ADDRESS "isActive()"

# 查询 Precompile 可用性
cast call --rpc-url $RPC_URL $PROXY_ADDRESS "isPrecompileAvailable()"
```

#### 权限管理
```bash
# 转移所有权
cast send --private-key $ADMIN_KEY --rpc-url $RPC_URL --legacy \
  --to $PROXY_ADDRESS "transferOwnership(address)" $NEW_OWNER

# 暂停合约
cast send --private-key $ADMIN_KEY --rpc-url $RPC_URL --legacy \
  --to $PROXY_ADDRESS "pause()"

# 恢复合约
cast send --private-key $ADMIN_KEY --rpc-url $RPC_URL --legacy \
  --to $PROXY_ADDRESS "unpause()"
```

#### 铸造操作
```bash
# 铸造代币
cast send --private-key $ADMIN_KEY --rpc-url $RPC_URL --legacy \
  --to $PROXY_ADDRESS "mint(address,uint256)" $TARGET_ADDRESS $AMOUNT

# 批量铸造
cast send --private-key $ADMIN_KEY --rpc-url $RPC_URL --legacy \
  --to $PROXY_ADDRESS "batchMint(address[],uint256[])" "[$ADDR1,$ADDR2]" "[$AMOUNT1,$AMOUNT2]"
```

#### 白名单管理
```bash
# 添加到销毁白名单
cast send --private-key $ADMIN_KEY --rpc-url $RPC_URL --legacy \
  --to $PROXY_ADDRESS "addBurnWhitelist(address)" $TARGET_ADDRESS

# 查询白名单状态
cast call --rpc-url $RPC_URL $PROXY_ADDRESS "isBurnAllowed(address)" $TARGET_ADDRESS

# 获取所有白名单地址
cast call --rpc-url $RPC_URL $PROXY_ADDRESS "getBurnWhitelist()"

# 销毁代币
cast send --private-key $ADMIN_KEY --rpc-url $RPC_URL --legacy \
  --to $PROXY_ADDRESS "burn(address,uint256)" $TARGET_ADDRESS $AMOUNT
```

## 安全考虑

### 权限控制

1. **双重验证**: Precompile 和智能合约都进行权限检查
2. **OpenZeppelin 标准**: 使用经过审计的权限管理合约
3. **紧急暂停**: 支持 pause/unpause 功能
4. **所有权转移**: 安全的所有权转移机制

### 输入验证

1. **Precompile 层**: 严格的输入长度和格式验证
2. **合约层**: 地址有效性和数量范围检查
3. **边界保护**: 防止销毁全部余额的保护机制

### 状态保护

1. **激活机制**: 支持延迟激活功能
2. **白名单控制**: 严格的销毁地址白名单管理
3. **可升级性**: 通过代理合约支持逻辑升级

### 升级安全

1. **透明代理**: 使用 OpenZeppelin 标准透明代理
2. **初始化保护**: 防止重复初始化
3. **存储布局**: 兼容的存储布局设计

## API 参考

### 核心函数

#### 管理函数
```solidity
function initialize(address _owner) external initializer
function transferOwnership(address newOwner) public virtual onlyOwner
function pause() external onlyOwner
function unpause() external onlyOwner
function setActivationBlock(uint256 _activationBlock) external onlyOwner
```

#### 代币操作
```solidity
function mint(address to, uint256 amount) external onlyOwner onlyActive whenNotPaused onlyWithPrecompile
function burn(address from, uint256 amount) external onlyOwner onlyActive whenNotPaused onlyWithPrecompile
function batchMint(address[] calldata recipients, uint256[] calldata amounts) external onlyOwner onlyActive whenNotPaused onlyWithPrecompile
```

#### 白名单管理
```solidity
function addBurnWhitelist(address account) external onlyOwner
function removeBurnWhitelist(address account) external onlyOwner
function batchAddBurnWhitelist(address[] calldata accounts) external onlyOwner
function isBurnAllowed(address account) public view returns (bool)
function getBurnWhitelist() external view returns (address[] memory)
```

#### 查询函数
```solidity
function owner() public view returns (address)
function isActive() public view returns (bool)
function paused() public view returns (bool)
function isPrecompileAvailable() public view returns (bool)
function VERSION() external pure returns (string memory)
```

### 事件

```solidity
event TokenMinted(address indexed to, uint256 amount);
event TokenBurned(address indexed from, uint256 amount);
event BurnWhitelistAdded(address indexed account);
event BurnWhitelistRemoved(address indexed account);
event ActivationBlockSet(uint256 activationBlock);
event Initialized(address indexed owner, uint256 activationBlock);
```

### 错误代码

#### Precompile 错误
- `"empty input"`: 输入数据为空
- `"missing operation data"`: 缺少操作数据
- `"invalid operation"`: 无效的操作码
- `"unauthorized: only contract manager can call"`: 未授权调用
- `"invalid data length for mint/burn"`: mint/burn 数据长度无效
- `"invalid amount"`: 无效数量
- `"insufficient balance"`: 余额不足

#### 合约错误
- `"Owner cannot be zero address"`: 所有者不能是零地址
- `"Cannot mint to zero address"`: 不能向零地址铸造
- `"Cannot burn from zero address"`: 不能从零地址销毁
- `"Amount must be greater than zero"`: 数量必须大于零
- `"Address is not in burn whitelist"`: 地址不在销毁白名单中
- `"Cannot burn entire balance"`: 不能销毁全部余额
- `"Precompile is not available"`: Precompile 不可用
- `"Token Manager is not active"`: Token Manager 未激活
- `"Precompile call failed"`: Precompile 调用失败

## 更新日志

### v1.4.0 (2024年12月)
- ✅ **新增 TEST_OP**: 添加专用的 Precompile 可用性测试操作码 (0x01)
- ✅ **操作码重新分配**: TEST_OP=0x01, MINT_OP=0x02, BURN_OP=0x03
- ✅ **优化权限检查**: TEST_OP 无需权限验证，提高可用性检测可靠性
- ✅ **改进可用性检测**: `isPrecompileAvailable()` 使用 TEST_OP 返回 "OK" 进行检测
- ✅ **简化输入验证**: Precompile 层面的输入验证更加精确和灵活

### v1.3.0 (2024年12月)
- ✅ **OpenZeppelin 集成**: 采用标准的 OwnableUpgradeable、PausableUpgradeable、TransparentUpgradeableProxy
- ✅ **简化架构**: 移除复杂的调用者检查机制，采用更直接的权限控制
- ✅ **标准化部署**: 使用 CREATE 操作码进行标准部署，移除 CREATE2 复杂性
- ✅ **白名单优化**: 允许 null 地址，移除重复的白名单检查函数
- ✅ **安全增强**: 余额保护逻辑移至合约层，增加 Precompile 可用性检查

### v1.2.0 (2024年11月)
- ✅ **CREATE2 支持**: 添加确定性地址部署支持
- ✅ **增强安全**: 添加调用者验证和更严格的输入检查
- ✅ **事件支持**: 完整的事件发射机制
- ✅ **批量操作**: 支持批量铸造和批量白名单管理

### v1.1.0 (2024年11月)
- ✅ **基础功能**: mint/burn 操作、白名单管理、权限控制
- ✅ **代理支持**: 基础的可升级代理合约
- ✅ **激活机制**: 可配置的激活块高度

### v1.0.0 (2024年10月)
- ✅ **核心架构**: Precompile + Smart Contract 双层架构
- ✅ **基本操作**: 基础的代币铸造和销毁功能 