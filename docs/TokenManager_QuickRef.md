# Token Manager 预编译合约快速参考

**⚠️ 重要限制：管理员地址无法更换**

由于预编译合约无法持久化 `SetState` 操作（只能持久化余额操作），所有状态必须硬编码。详细技术说明请参考完整文档。

## 🚀 快速开始

### 合约地址
```
0x0000000000000000000000000000000000000101
```

### 管理员地址
```
0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15
```

## 📝 操作速查

### 1. 查询当前管理员
```bash
cast call 0x0000000000000000000000000000000000000101 0x20 --rpc-url http://127.0.0.1:8123
```

### 2. 增发代币
```bash
# 使用编码工具
python3 scripts/token_manager_encoder.py mint 0xb6c11e83a19893a0de12ae7b77ff224eae7ea8cb 10

# 直接使用 cast（给 0xb6c11e83a19893a0de12ae7b77ff224eae7ea8cb 增发 10 个代币）
cast send 0x0000000000000000000000000000000000000101 \
  "0x01000000000000000000000000b6c11e83a19893a0de12ae7b77ff224eae7ea8cb0000000000000000000000000000000000000000000000008AC7230489E80000" \
  --private-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
  --rpc-url http://127.0.0.1:8123
```

### 3. 销毁代币
```bash
# 使用编码工具
python3 scripts/token_manager_encoder.py burn 0x1f50d8C07D68F2Ec566a00Ca0689a9B24799D986 5

# 直接使用 cast（销毁 0x1f50d8C07D68F2Ec566a00Ca0689a9B24799D986 地址的 5 个代币）
cast send 0x0000000000000000000000000000000000000101 \
  "0x020000000000000000000000001f50d8C07D68F2Ec566a00Ca0689a9B24799D986000000000000000000000000000000000000000000000000045639182B5AF0000" \
  --private-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
  --rpc-url http://127.0.0.1:8123
```

**⚠️ 重要限制：**
- 无法销毁地址的全部余额（防止节点崩溃）
- 每个地址必须保留至少 1 wei
- 尝试全量销毁会返回 `insufficient balance for burn` 错误



### 4. 检查余额
```bash
cast balance 0xb6c11e83a19893a0de12ae7b77ff224eae7ea8cb --rpc-url http://127.0.0.1:8123
```

### 5. 运行完整测试
```bash
./scripts/token_manager_test.sh
```

## 🔧 配置要求

### 链配置 (chainspec.json)
```json
{
  "tokenManager": {
    "activationBlock": 50,
    "burnAuthorizedAddresses": [
      "0x1f50d8C07D68F2Ec566a00Ca0689a9B24799D986",
      "0x0000000000000000000000000000000000000000"
    ]
  }
}
```

### EIP-1559 支持
```json
{
  "londonBlock": 0
}
```

## ⚠️ 常见问题

### "Token Manager not activated"
- 检查当前区块高度是否 >= `activationBlock`
- 确认 `tokenManager` 配置存在

### "invalid token operation data"
- 数据格式：`[操作码 1字节][地址 32字节][数量 32字节]`
- 地址和数量都需要32字节填充

### "unauthorized: only current admin"
- 确认使用正确的管理员私钥
- 检查当前管理员地址

### "insufficient balance for burn"
- 无法销毁地址的全部余额（防止节点崩溃）
- 每个地址必须保留至少 1 wei

## 💰 Gas 费用

| 操作 | Gas 消耗 | 说明 |
|------|----------|------|
| 增发 (Mint) | 0 | **免费操作** |
| 销毁 (Burn) | 15000 | 正常收费 |
| 查询管理员 | 10000 | 正常收费 |

## 🎯 操作码速查

| 代码 | 操作 | 格式 |
|------|------|------|
| `0x01` | 增发 | `01 + 32字节地址 + 32字节数量` |
| `0x02` | 销毁 | `02 + 32字节地址 + 32字节数量` |
| `0x20` | 查管理员 | `20` |

## 🛠️ 工具使用

### 数据编码工具
```bash
# 查看帮助
python3 scripts/token_manager_encoder.py --help

# 示例
python3 scripts/token_manager_encoder.py mint 0xADDRESS 10
python3 scripts/token_manager_encoder.py burn 0xADDRESS 5
python3 scripts/token_manager_encoder.py query_admin
```

### 测试脚本
```bash
# 查看帮助
./scripts/token_manager_test.sh --help

# 运行所有测试
./scripts/token_manager_test.sh

# 使用自定义 RPC
RPC_URL=http://localhost:8545 ./scripts/token_manager_test.sh
``` 