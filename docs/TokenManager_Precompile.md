# Token Manager 预编译合约技术文档

## 概述

Token Manager 是 xlayer-erigon L2 链上的一个原生预编译合约，用于实现 native gas token 的增发和销毁功能。该合约通过 L2 交易调用，提供高性能的代币管理能力。

**🔐 多签管理员设计：采用硬编码的多管理员地址，要求超过半数签名验证。**

**⚠️ 重要说明：管理员地址列表是硬编码的，无法在运行时更换。**

**技术原因说明：**
预编译合约在 Erigon 中有特殊的地址空间（如 `0x0000000000000000000000000000000000000101`），这些地址被系统识别为预编译功能而非普通智能合约。虽然预编译合约可以修改账户余额（通过 `AddBalance`/`SubBalance`），但无法使用合约存储功能（`SetState`/`GetState`）来持久化自定义状态。

具体来说：
- ✅ **余额操作正常**：`intraBlockState.AddBalance()` 和 `SubBalance()` 可以正常工作并持久化
- ❌ **状态存储失效**：`intraBlockState.SetState()` 调用成功但数据无法持久化到数据库
- ❌ **合约存储隔离**：预编译地址不被当作真正的合约账户来处理存储层操作

因此，所有需要持久化的状态（如管理员地址）都必须硬编码在合约代码中，无法通过链上存储来动态修改。

## 设计理念

### 核心原则
1. **安全性优先**: 只有授权管理员可以执行关键操作
2. **多签验证**: 要求超过半数的管理员签名才能执行操作
3. **权限控制**: 销毁操作限制在预授权地址列表中
4. **固定管理员**: 使用硬编码管理员地址列表，简化权限管理
5. **无状态设计**: 预编译合约不依赖链上状态存储
6. **事件日志**: 所有关键操作都发出标准以太坊事件
7. **共识安全**: 在激活高度前保持与旧节点的共识兼容性

### 架构特点
- **预编译合约**: 直接集成在 EVM 中，高性能执行
- **ForkID 激活**: 通过 ForkID 映射在指定区块高度激活
- **链配置集成**: 配置存储在 chain config 中
- **状态存储**: 使用链上存储保存管理员状态

## 功能设计

### 操作类型

| 操作码 | 名称 | 功能 | 权限要求 |
|--------|------|------|----------|
| `0x01` | TOKEN_MINT_OP | 增发代币到指定地址 | 多签验证 (≥50% 管理员签名) |
| `0x02` | TOKEN_BURN_OP | 销毁指定地址的代币 | 多签验证 (≥50% 管理员签名) + 防崩溃保护 |
| `0x20` | QUERY_ADMIN_OP | 查询管理员地址列表 | 无限制 |

### 安全机制与限制

#### 1. Burn 操作保护机制

**⚠️ 关键安全限制：Token Manager 实施了防止全量销毁的保护机制，以避免节点崩溃。**

**技术原理：**
```go
// 检查余额是否足够，防止全量销毁
currentBalance := c.evm.intraBlockState.GetBalance(targetAddress)
if currentBalance.Cmp(amount) <= 0 {
    return nil, errors.New("insufficient balance for burn")
}
```

**保护机制说明：**
- 如果要销毁的数量 **大于等于** 当前余额，操作将被拒绝
- 这确保了任何地址的余额都不能被完全清零
- 防止了因余额为零而导致的底层系统崩溃

**实际测试验证：**
- ✅ **有保护时**：尝试全量burn返回 `insufficient balance for burn` 错误
- ❌ **无保护时**：全量burn会导致节点直接崩溃，需要重启

**适用范围：**
- **所有地址类型**：普通账户、预编译合约地址、null地址等
- **任何数量**：只要 `burnAmount >= currentBalance` 就会被拦截
- **管理员操作**：即使是管理员也无法绕过此限制

#### 2. 最小余额保留

**实际效果：**
- 每个地址必须保留至少 1 wei 的余额
- 可以销毁 `currentBalance - 1` wei，但不能全部销毁
- 这个设计是出于系统稳定性考虑，不是业务逻辑需求

**示例：**
```bash
# 地址当前余额：1000000000000000000 wei (1 ETH)
# 可以销毁：   999999999999999999 wei (0.999999999999999999 ETH)
# 必须保留：   至少 1 wei

# ✅ 成功的操作
cast send 0x0101 "0x02...999999999999999999" # burn 0.999999999999999999 ETH

# ❌ 失败的操作  
cast send 0x0101 "0x02...1000000000000000000" # 尝试 burn 全部 1 ETH
# 返回：insufficient balance for burn
```

#### 3. 权限验证

**管理员验证：**
- 只有硬编码的管理员地址可以执行 mint/burn 操作
- 其他地址调用将返回 `unauthorized` 错误

**地址授权：**
- 包含目标地址白名单验证

### 管理员机制

#### 多签管理员配置
```go
// 硬编码的管理员地址列表
var ADMIN_ADDRESSES = []libcommon.Address{
    libcommon.HexToAddress("0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15"),
    libcommon.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
    // 可以根据需要添加更多管理员地址
}

// 最小签名数量要求 (当前要求超过50%)
var MIN_SIGNATURES = uint64(2)  // 对于2个管理员，需要2个签名
```

#### 多签验证流程
1. **签名生成**: 每个管理员使用私钥对操作数据签名
2. **签名验证**: 预编译合约验证签名数量是否达到最小要求
3. **权限检查**: 确认签名者都是有效的管理员地址
4. **操作执行**: 验证通过后执行相应的mint/burn操作

#### 管理员生命周期
1. **固定管理员**: 使用硬编码的管理员地址列表，不可更换
2. **永久有效**: 管理员地址永远有效，无法被禁用或更换
3. **多签安全**: 单个管理员无法独立执行操作，增强安全性

### 事件日志

#### TokenMinted 事件
```solidity
event TokenMinted(address indexed to, uint256 amount, address indexed admin);
```
- **签名**: `0xab8530f87dc9b59234c4623bf917212bb2536d647574c8e7e5da92c2ede0c9f8`

#### TokenBurned 事件
```solidity
event TokenBurned(address indexed from, uint256 amount, address indexed admin);
```
- **签名**: `0xcc16f5dbb4873280815c1ee09dbd06736cffcc184412cf7a71a0fdb75d397ca5`



## 技术限制与约束

### 预编译合约存储限制

**核心问题：** 预编译合约无法使用智能合约的存储功能来持久化自定义状态。

**具体表现：**

1. **状态存储 API 失效**
   ```go
   // 这些调用在预编译合约中无法持久化
   c.evm.intraBlockState.SetState(contractAddr, &key, value)  // ❌ 调用成功但不持久化
   c.evm.intraBlockState.GetState(contractAddr, &key, &result) // ❌ 读取时返回空值
   ```

2. **余额操作正常**
   ```go
   // 这些调用在预编译合约中正常工作
   c.evm.intraBlockState.AddBalance(address, amount)  // ✅ 正常持久化
   c.evm.intraBlockState.SubBalance(address, amount)  // ✅ 正常持久化
   ```

3. **系统层面原因**
   - 预编译地址（如 `0x0101`）被 EVM 识别为系统功能，而非用户合约
   - 账户余额属于账户层面，可以正常修改
   - 合约存储属于合约层面，对预编译地址被特殊处理或忽略

**解决方案：**
- 所有需要持久化的状态都必须硬编码在合约代码中
- 动态配置只能通过 `chain.Config` 在启动时加载
- 无法实现运行时状态变更（如管理员切换）

### 设计权衡

**优势：**
- 简化了权限管理逻辑
- 避免了复杂的状态同步问题
- 提高了安全性（无法意外修改关键配置）

**劣势：**
- 管理员地址固定，无法灵活调整
- 配置变更需要代码升级
- 降低了运营灵活性

## 技术实现

### 合约地址
```
0x0000000000000000000000000000000000000101
```

### Gas 消耗
```go
// Gas costs defined in params/protocol_params.go
TokenMintGas   uint64 = 0      // 增发操作（免费）
TokenBurnGas   uint64 = 15000  // 销毁操作
TokenQueryGas  uint64 = 10000  // 查询操作
```

**⚠️ 重要说明：**
- **Mint 操作免费**：为了鼓励代币增发，mint操作不消耗额外gas
- **Burn 操作收费**：销毁操作仍需正常gas费用
- **Query 操作收费**：查询操作需要少量gas费用

### 数据编码格式

#### 增发/销毁操作
```
[1 byte: 操作码][32 bytes: 目标地址(前12字节为0填充)][32 bytes: 数量]
```

#### 查询管理员
```
[1 byte: 操作码]
```

### 核心代码结构

#### 主要文件
- `core/vm/contracts_tokenManager.go` - 预编译合约实现
- `core/vm/contracts_zkevm.go` - 预编译合约映射
- `erigon-lib/chain/chain_config_okx.go` - 配置结构定义
- `params/protocol_params.go` - Gas 费用定义

#### 关键接口
```go
type PrecompiledContract_zkEvm interface {
    RequiredGas(input []byte) uint64
    Run(input []byte) ([]byte, error)
    SetCounterCollector(cc *CounterCollector)
    SetOutputLength(outLength int)
    SetEVM(evm *EVM)
}
```

## 配置管理

### 链配置格式
```json
{
  "tokenManager": {
    "activationBlock": 100,
    "burnAuthorizedAddresses": [
      "0x1f50d8C07D68F2Ec566a00Ca0689a9B24799D986",
      "0x0000000000000000000000000000000000000000"
    ]
  }
}
```

### 配置说明
- `activationBlock`: 预编译合约激活的区块高度
- `burnAuthorizedAddresses`: 允许被销毁代币的地址列表

## 多签数据格式

### Calldata 结构

多签操作的 Calldata 格式如下：

| 字段 | 长度 | 描述 |
|------|------|------|
| 操作码 | 1 byte | 0x01 (mint) 或 0x02 (burn) |
| 目标地址 | 32 bytes | 32字节对齐的地址 (前12字节为0) |
| 数量 | 32 bytes | uint256 格式的代币数量 |
| Nonce | 8 bytes | 防重放攻击的随机数 |
| 签名数量 | 1 byte | 包含的签名数量 |
| 签名1 | 65 bytes | 第一个管理员的签名 (r+s+v) |
| 签名2 | 65 bytes | 第二个管理员的签名 (r+s+v) |
| ... | 65 bytes | 更多签名 (如果有) |

**总长度**: 1 + 32 + 32 + 8 + 1 + (65 × 签名数量) = 74 + (65 × N) bytes

### 签名消息格式

每个管理员需要对以下数据进行签名：

```go
// 消息哈希构造
data := []byte{}
data = append(data, operation)           // 1 byte: 操作码
data = append(data, target.Bytes()...)   // 20 bytes: 目标地址
data = append(data, amount.PaddedBytes(32)...) // 32 bytes: 数量
data = append(data, nonceBytes...)       // 8 bytes: nonce (big-endian)

messageHash := crypto.Keccak256(data)
signature := crypto.Sign(messageHash, privateKey)
```

### Go 生成脚本

为了简化多签 Calldata 的生成，我们提供了一个 Go 脚本，位于 `cmd/multisig_gen/main.go`：

#### 脚本功能
- 自动生成正确格式的多签 Calldata
- 支持 mint 和 burn 操作
- 使用真实的管理员私钥进行签名
- 输出完整的 cast 命令用于测试

#### 使用方法

1. **准备环境**
   ```bash
   cd cmd/multisig_gen
   ```

2. **运行脚本**
   ```bash
   go run main.go
   ```

3. **输出示例**
   ```bash
   === MINT 10 ETH ===
   Amount: 10000000000000000000 wei
   Nonce: 1
   Cast Command:
   cast send 0x0000000000000000000000000000000000000101 "0x01..." --private-key 0x... --rpc-url http://127.0.0.1:8123 --legacy
   
   === BURN 5 ETH ===
   Amount: 5000000000000000000 wei
   Nonce: 2
   Cast Command:
   cast send 0x0000000000000000000000000000000000000101 "0x02..." --private-key 0x... --rpc-url http://127.0.0.1:8123 --legacy
   ```

#### 脚本配置

脚本中的关键配置可以根据需要修改：

```go
// 目标地址：要接收mint/burn的地址
target := libcommon.HexToAddress("0x0000000000000000000000000000000000000000")

// 管理员私钥 (对应具体的管理员地址)
admin1PrivateKey := "0x..."  // 对应 0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15
admin2PrivateKey := "0x..."  // 对应 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
```

#### 核心函数

```go
// createMessageHash 复制 tokenManager 中的确切逻辑
func createMessageHash(operation byte, target libcommon.Address, amount *uint256.Int, nonce uint64) []byte {
    data := make([]byte, 0, 1+20+32+8)
    data = append(data, operation)
    data = append(data, target.Bytes()...)
    data = append(data, amount.PaddedBytes(32)...)
    
    nonceBytes := make([]byte, 8)
    for i := 0; i < 8; i++ {
        nonceBytes[7-i] = byte(nonce >> (i * 8))
    }
    data = append(data, nonceBytes...)
    
    return crypto.Keccak256(data)
}

func signMessage(privateKey *ecdsa.PrivateKey, msgHash []byte) ([]byte, error) {
    signature, err := crypto.Sign(msgHash, privateKey)
    if err != nil {
        return nil, err
    }
    return signature, nil
}
```

#### 测试验证

脚本已验证支持的地址类型：
- ✅ **普通账户**: 如 `0xAeFA44f2E8cb4871A0cA862a4E7C5f2761111886`
- ✅ **NULL地址**: `0x0000000000000000000000000000000000000000`
- ✅ **预编译地址**: 如 `0x0000000000000000000000000000000000000001` (ecrecover)

## 使用指南

### Cast 命令行工具

#### 查询当前管理员
```bash
cast call 0x0000000000000000000000000000000000000101 0x20 --rpc-url http://127.0.0.1:8123
```

#### 增发代币
```bash
# 给 0xb6c11e83a19893a0de12ae7b77ff224eae7ea8cb 增发 10 个代币
cast send 0x0000000000000000000000000000000000000101 \
  "0x01000000000000000000000000b6c11e83a19893a0de12ae7b77ff224eae7ea8cb0000000000000000000000000000000000000000000000008AC7230489E80000" \
  --private-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
  --rpc-url http://127.0.0.1:8123
```

#### 销毁代币
```bash
# 销毁 0x1f50d8C07D68F2Ec566a00Ca0689a9B24799D986 地址的 5 个代币
cast send 0x0000000000000000000000000000000000000101 \
  "0x020000000000000000000000001f50d8C07D68F2Ec566a00Ca0689a9B24799D98600000000000000000000000000000000000000000000000045639182B5AF0000" \
  --private-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
  --rpc-url http://127.0.0.1:8123
```



### Postman/cURL 示例

#### 查询管理员地址
```bash
curl --location 'http://127.0.0.1:8123' \
--header 'Content-Type: application/json' \
--data '{
  "jsonrpc": "2.0",
  "method": "eth_call",
  "params": [
    {
      "to": "0x0000000000000000000000000000000000000101",
      "data": "0x20"
    },
    "latest"
  ],
  "id": 1
}'
```

#### 检查余额
```bash
curl --location 'http://127.0.0.1:8123' \
--header 'Content-Type: application/json' \
--data '{
  "jsonrpc": "2.0",
  "method": "eth_getBalance",
  "params": [
    "0xb6c11e83a19893a0de12ae7b77ff224eae7ea8cb",
    "latest"
  ],
  "id": 1
}'
```

## 测试脚本

### 数据编码辅助脚本
```python
#!/usr/bin/env python3
"""Token Manager 数据编码辅助工具"""

def encode_mint_data(target_address, amount_wei):
    """编码增发操作数据"""
    # 操作码: 0x01
    op = "01"
    # 地址: 32字节，前12字节为0填充
    addr = target_address.replace("0x", "").lower().zfill(64)
    # 数量: 32字节
    amount = hex(amount_wei)[2:].zfill(64)
    return f"0x{op}{addr}{amount}"

def encode_burn_data(target_address, amount_wei):
    """编码销毁操作数据"""
    # 操作码: 0x02
    op = "02"
    # 地址: 32字节，前12字节为0填充
    addr = target_address.replace("0x", "").lower().zfill(64)
    # 数量: 32字节
    amount = hex(amount_wei)[2:].zfill(64)
    return f"0x{op}{addr}{amount}"

def encode_change_admin_data(new_admin_address):
    """编码更换管理员操作数据"""
    # 操作码: 0x10
    op = "10"
    # 地址: 32字节，前12字节为0填充
    addr = new_admin_address.replace("0x", "").lower().zfill(64)
    return f"0x{op}{addr}"

def encode_query_admin_data():
    """编码查询管理员操作数据"""
    return "0x20"

# 示例用法
if __name__ == "__main__":
    # 增发 10 个代币到指定地址
    mint_data = encode_mint_data(
        "0xb6c11e83a19893a0de12ae7b77ff224eae7ea8cb", 
        10 * 10**18
    )
    print(f"Mint data: {mint_data}")
    
    # 销毁 5 个代币
    burn_data = encode_burn_data(
        "0x1f50d8C07D68F2Ec566a00Ca0689a9B24799D986", 
        5 * 10**18
    )
    print(f"Burn data: {burn_data}")
    
    # 更换管理员
    change_admin_data = encode_change_admin_data(
        "0x1234567890123456789012345678901234567890"
    )
    print(f"Change admin data: {change_admin_data}")
    
    # 查询管理员
    query_data = encode_query_admin_data()
    print(f"Query admin data: {query_data}")
```

### 完整测试流程脚本
```bash
#!/bin/bash
# Token Manager 功能测试脚本

# 配置
RPC_URL="http://127.0.0.1:8123"
ADMIN_PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
CONTRACT_ADDRESS="0x0000000000000000000000000000000000000101"
TEST_ADDRESS="0xb6c11e83a19893a0de12ae7b77ff224eae7ea8cb"

echo "=== Token Manager 功能测试 ==="

# 1. 查询当前管理员
echo "1. 查询当前管理员地址..."
ADMIN_RESULT=$(cast call $CONTRACT_ADDRESS 0x20 --rpc-url $RPC_URL)
echo "当前管理员: $ADMIN_RESULT"

# 2. 检查目标地址初始余额
echo "2. 检查目标地址初始余额..."
INITIAL_BALANCE=$(cast balance $TEST_ADDRESS --rpc-url $RPC_URL)
echo "初始余额: $INITIAL_BALANCE"

# 3. 执行增发操作 (10 个代币)
echo "3. 执行增发操作..."
MINT_DATA="0x01000000000000000000000000b6c11e83a19893a0de12ae7b77ff224eae7ea8cb0000000000000000000000000000000000000000000000008AC7230489E80000"
MINT_TX=$(cast send $CONTRACT_ADDRESS $MINT_DATA \
  --private-key $ADMIN_PRIVATE_KEY \
  --rpc-url $RPC_URL \
  --json)

if [ $? -eq 0 ]; then
    echo "增发交易成功!"
    MINT_TX_HASH=$(echo $MINT_TX | jq -r '.transactionHash')
    echo "交易哈希: $MINT_TX_HASH"
    
    # 获取交易回执
    echo "获取交易日志..."
    cast receipt $MINT_TX_HASH --rpc-url $RPC_URL
else
    echo "增发交易失败!"
    exit 1
fi

# 4. 检查增发后余额
echo "4. 检查增发后余额..."
FINAL_BALANCE=$(cast balance $TEST_ADDRESS --rpc-url $RPC_URL)
echo "最终余额: $FINAL_BALANCE"

# 5. 计算余额变化
echo "5. 余额变化分析..."
BALANCE_DIFF=$((FINAL_BALANCE - INITIAL_BALANCE))
echo "余额增加: $BALANCE_DIFF wei"
echo "预期增加: 10000000000000000000 wei (10 ETH)"

if [ $BALANCE_DIFF -eq 10000000000000000000 ]; then
    echo "✅ 增发功能测试通过!"
else
    echo "❌ 增发功能测试失败!"
fi

echo "=== 测试完成 ==="
```

## 测试验证与发现

### 关键安全验证

#### Burn 保护机制验证

通过实际测试确认了 Token Manager 的安全保护机制：

**✅ 测试场景 1：正常 Burn 操作**
```bash
# 测试地址：0xe45Cd8b6Ce50B3d26A3aDB97000eF999AD044Ad4
# 初始余额：0 wei
# 操作：mint 1 ETH -> burn 0.5 ETH
# 结果：✅ 成功，最终余额 0.5 ETH
```

**✅ 测试场景 2：保护机制生效**  
```bash
# 测试地址：0xAeFA44f2E8cb4871A0cA862a4E7C5f2761111886
# 操作：mint 1 ETH -> 尝试 burn 1 ETH (全部)
# 结果：❌ 正确拒绝，返回 "insufficient balance for burn"
```

**❌ 测试场景 3：无保护机制的危险性**
```bash
# 移除保护机制后的测试
# 操作：mint 1 ETH -> burn 1 ETH (全部)  
# 结果：🚨 节点直接崩溃，需要重启
```

#### 关键发现总结

1. **保护机制必要性确认**
   - 有保护：安全运行，返回错误信息
   - 无保护：节点崩溃，系统不稳定

2. **数学逻辑验证**
   - Mint 操作：余额精确增加对应数量
   - Burn 操作：余额精确减少对应数量  
   - 保护检查：`currentBalance.Cmp(amount) <= 0` 有效工作

3. **地址兼容性确认**
   - ✅ 普通账户地址：完全支持
   - ✅ 预编译合约地址：完全支持
   - ✅ null 地址 (0x00...00)：完全支持
   - ✅ Token Manager 自身地址：支持 mint

4. **Gas 费用验证**
   - Mint 操作：0 gas（免费）
   - Burn 操作：正常 gas 消耗
   - Query 操作：正常 gas 消耗

### 生产环境建议

1. **保护机制**：绝对不要移除 burn 保护机制
2. **操作限制**：接受每个地址必须保留 1 wei 的限制
3. **错误处理**：正确处理 `insufficient balance for burn` 错误
4. **监控建议**：监控 burn 操作失败情况，避免意外的全量销毁尝试

**🎯 结论：Token Manager 的安全保护机制设计合理，有效防止了系统崩溃，是生产环境的必要安全措施。**

## 故障排除

### 常见错误及解决方案

#### 1. "Token Manager not activated"
**原因**: 当前区块高度未达到激活高度
**解决**: 检查 `activationBlock` 配置和当前区块高度

#### 2. "invalid token operation data"
**原因**: 调用数据格式不正确
**解决**: 确保数据长度正确，地址和数量都是32字节编码

#### 3. "unauthorized: only current admin"
**原因**: 调用者不是当前管理员
**解决**: 使用正确的管理员私钥签名交易

#### 4. "address not authorized for burn"
**原因**: 目标地址不在销毁授权列表中
**解决**: 将目标地址添加到 `burnAuthorizedAddresses` 配置中

### 调试技巧

1. **查看事件日志**: 使用 `cast receipt` 查看交易回执中的事件
2. **验证数据编码**: 使用提供的 Python 脚本验证调用数据格式
3. **检查权限**: 确认调用者地址和当前管理员地址
4. **监控余额变化**: 在操作前后检查相关地址余额

## 安全考虑

### 权限管理
- 管理员私钥必须安全保管
- 管理员地址是固定的，无法更换
- 确保管理员私钥的安全性至关重要

### 配置安全
- 销毁授权列表应最小化
- 定期审核授权地址列表
- 谨慎设置激活区块高度

### 共识安全
- 确保所有节点在激活高度前同步升级
- 测试不同节点版本的兼容性
- 监控网络共识状态

## 版本历史

### v1.1.0 (当前版本)
- 实现基本的增发/销毁功能
- 使用固定的硬编码管理员地址
- 移除了管理员变更功能（因为预编译合约无法持久化 `SetState` 操作）
- 集成事件日志
- 支持激活高度控制

### v1.0.0 
- 实现基本的增发/销毁功能
- 支持管理员管理（已移除）
- 集成事件日志
- 支持激活高度控制

## 参考资料

- [Erigon 架构文档](../programmers_guide/)
- [以太坊预编译合约规范](https://eips.ethereum.org/EIPS/eip-1352)
- [EVM 深度剖析](https://ethereum.github.io/yellowpaper/paper.pdf) 