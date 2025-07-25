# Token Manager 预编译合约技术文档

## 概述

Token Manager 是 xlayer-erigon L2 链上的一个原生预编译合约，用于实现 native gas token 的增发和销毁功能。该合约通过 L2 交易调用，提供高性能的代币管理能力。

## 设计理念

### 核心原则
1. **安全性优先**: 只有授权管理员可以执行关键操作
2. **权限控制**: 销毁操作限制在预授权地址列表中
3. **管理员生命周期**: 支持管理员更换，老管理员永久失效
4. **持久化存储**: 管理员地址在链重启后保持不变
5. **事件日志**: 所有关键操作都发出标准以太坊事件
6. **共识安全**: 在激活高度前保持与旧节点的共识兼容性

### 架构特点
- **预编译合约**: 直接集成在 EVM 中，高性能执行
- **ForkID 激活**: 通过 ForkID 映射在指定区块高度激活
- **链配置集成**: 配置存储在 chain config 中
- **状态存储**: 使用链上存储保存管理员状态

## 功能设计

### 操作类型

| 操作码 | 名称 | 功能 | 权限要求 |
|--------|------|------|----------|
| `0x01` | TOKEN_MINT_OP | 增发代币到指定地址 | 仅管理员 |
| `0x02` | TOKEN_BURN_OP | 销毁指定地址的代币 | 仅管理员 + 目标地址在授权列表 |
| `0x10` | CHANGE_ADMIN_OP | 更换管理员地址 | 仅当前管理员 |
| `0x20` | QUERY_ADMIN_OP | 查询当前管理员地址 | 无限制 |

### 管理员机制

#### 初始管理员
```go
var INITIAL_ADMIN = libcommon.HexToAddress("0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15")
```

#### 管理员生命周期
1. **初始状态**: 使用硬编码的初始管理员地址
2. **更换管理员**: 当前管理员可以指定新管理员
3. **永久失效**: 一旦更换，旧管理员（包括初始管理员）永久失效
4. **持久化存储**: 管理员地址存储在链上，重启后保持不变

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

#### AdminChanged 事件
```solidity
event AdminChanged(address indexed oldAdmin, address indexed newAdmin);
```
- **签名**: `0x7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f`

## 技术实现

### 合约地址
```
0x0000000000000000000000000000000000000101
```

### Gas 消耗
```go
// Gas costs defined in params/protocol_params.go
TokenMintGas   uint64 = 20000  // 增发操作
TokenBurnGas   uint64 = 15000  // 销毁操作
TokenAdminGas  uint64 = 25000  // 管理员操作
TokenQueryGas  uint64 = 10000  // 查询操作
```

### 数据编码格式

#### 增发/销毁操作
```
[1 byte: 操作码][32 bytes: 目标地址(前12字节为0填充)][32 bytes: 数量]
```

#### 更换管理员
```
[1 byte: 操作码][32 bytes: 新管理员地址(前12字节为0填充)]
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

#### 更换管理员
```bash
# 将管理员更换为 0x1234567890123456789012345678901234567890
cast send 0x0000000000000000000000000000000000000101 \
  "0x100000000000000000000000001234567890123456789012345678901234567890" \
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
- 定期轮换管理员地址
- 使用多重签名钱包管理管理员权限

### 配置安全
- 销毁授权列表应最小化
- 定期审核授权地址列表
- 谨慎设置激活区块高度

### 共识安全
- 确保所有节点在激活高度前同步升级
- 测试不同节点版本的兼容性
- 监控网络共识状态

## 版本历史

### v1.0.0 (当前版本)
- 实现基本的增发/销毁功能
- 支持管理员管理
- 集成事件日志
- 支持激活高度控制

## 参考资料

- [Erigon 架构文档](../programmers_guide/)
- [以太坊预编译合约规范](https://eips.ethereum.org/EIPS/eip-1352)
- [EVM 深度剖析](https://ethereum.github.io/yellowpaper/paper.pdf) 