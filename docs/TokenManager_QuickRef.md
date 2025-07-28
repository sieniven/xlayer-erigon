# Token Manager 预编译合约快速参考

**🔐 多签管理员：需要超过50%管理员签名**

由于预编译合约无法持久化 `SetState` 操作（只能持久化余额操作），所有状态必须硬编码。详细技术说明请参考完整文档。

## 🚀 快速开始

### 合约地址
```
0x0000000000000000000000000000000000000101
```

### 管理员地址 (多签)
```
0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15  # 管理员1
0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266  # 管理员2
```

### 签名要求
- **最小签名数**: 2 (需要所有管理员签名)
- **验证方式**: ECDSA 签名恢复验证

## 🔗 多签数据格式

### Calldata 结构
```
操作码(1) + 地址(32) + 数量(32) + Nonce(8) + 签名数(1) + 签名1(65) + 签名2(65) = 204 bytes
```

### Go 生成脚本位置
```
cmd/multisig_gen/main.go
```

**使用方法**: `cd cmd/multisig_gen && go run main.go`

## 📝 操作速查

### 1. 查询管理员列表
```bash
cast call 0x0000000000000000000000000000000000000101 0x20 --rpc-url http://127.0.0.1:8123
```

### 2. 生成多签交易 (推荐)
```bash
# 使用 Go 脚本生成多签 calldata
cd cmd/multisig_gen
go run main.go

# 输出完整的 cast 命令，直接复制执行即可
```

### 3. 多签增发代币 (示例)
```bash
# 给 NULL 地址增发 10 ETH (需要2个管理员签名)
cast send 0x0000000000000000000000000000000000000101 \
  "0x0100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008ac7230489e80000000000000000000102[签名1][签名2]" \
  --private-key 0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9 \
  --rpc-url http://127.0.0.1:8123 --legacy
```

### 4. 多签销毁代币 (示例)
```bash
# 从 NULL 地址销毁 5 ETH (需要2个管理员签名)
cast send 0x0000000000000000000000000000000000000101 \
  "0x0200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004563918244f40000000000000000000202[签名1][签名2]" \
  --private-key 0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9 \
  --rpc-url http://127.0.0.1:8123 --legacy
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