#!/bin/bash

# Token Manager Contracts Deployment Script
# This script deploys the Token Manager system using standard CREATE deployment
set -e

echo "🚀 Token Manager Deployment Script"
echo "=================================="
echo "📋 Features: Mint/Burn + OpenZeppelin Security"
echo ""

# 配置参数
PRIVATE_KEY="${PRIVATE_KEY:-0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9}"
RPC_URL="${RPC_URL:-http://localhost:8123}"
GAS_PRICE="${GAS_PRICE:-1000000000}"
GAS_LIMIT="${GAS_LIMIT:-5000000}"
MAX_WAIT_SECONDS=60 # 最大等待确认时间

# 管理员地址
OWNER_ADMIN="${OWNER_ADMIN:-0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15}"
PROXY_ADMIN="${PROXY_ADMIN:-0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15}"

# 激活配置
ACTIVATION_BLOCK="${ACTIVATION_BLOCK:-0}" # 默认设置为0，即立即激活

echo "📊 Deployment Configuration:"
echo "  RPC URL: $RPC_URL"
echo "  Owner/Admin: $OWNER_ADMIN"
echo "  Proxy Admin: $PROXY_ADMIN"
echo "  Gas Price: $GAS_PRICE"
echo "  Gas Limit: $GAS_LIMIT"
echo ""

# 验证环境变量
if [[ "$PRIVATE_KEY" == "0xYOUR_PRIVATE_KEY" || -z "$PRIVATE_KEY" ]]; then
    echo "❌ 错误：PRIVATE_KEY 未设置或无效"
    exit 1
fi

# 检查网络连接
echo "🔍 检查网络连接..."
if ! cast chain-id --rpc-url "$RPC_URL" > /dev/null; then
    echo "❌ 错误：无法连接到 RPC 端点 $RPC_URL"
    exit 1
fi

CHAIN_ID=$(cast chain-id --rpc-url "$RPC_URL")
BLOCK_NUMBER=$(cast block-number --rpc-url "$RPC_URL")
echo "✅ 已连接到 Chain ID: $CHAIN_ID, 区块高度: $BLOCK_NUMBER"
echo ""

# 检查部署者余额
DEPLOYER=$(cast wallet address --private-key "$PRIVATE_KEY")
BALANCE=$(cast balance "$DEPLOYER" --rpc-url "$RPC_URL" --ether)
echo "👛 部署者地址: $DEPLOYER"
echo "💰 余额: $BALANCE ETH"

if (( $(echo "$BALANCE < 0.01" | bc -l) )); then
    echo "❌ 错误：部署者余额不足 ($BALANCE ETH，至少需要 0.01 ETH)"
    exit 1
fi
echo ""

# 编译合约（保持不变）
echo "🔧 编译合约..."
cd ../contracts || { echo "❌ 错误：未找到合约目录"; exit 1; }

if ! command -v solc &> /dev/null; then
    echo "❌ 错误：未找到 solc 编译器，请安装 Solidity 编译器"
    exit 1
fi

# 编译实现合约
echo "  编译 TokenManagerV1.sol..."
if ! solc --bin --evm-version paris TokenManagerV1.sol -o . --overwrite --base-path . --include-path node_modules/ > compile_output.txt 2>&1; then
    echo "❌ 错误：编译 TokenManagerV1.sol 失败"
    cat compile_output.txt
    rm -f compile_output.txt
    exit 1
fi
rm -f compile_output.txt

# 编译代理合约
echo "  编译 TokenManagerProxy.sol..."
if ! solc --bin --evm-version paris TokenManagerProxy.sol -o . --overwrite --base-path . --include-path node_modules/ > compile_output.txt 2>&1; then
    echo "❌ 错误：编译 TokenManagerProxy.sol 失败"
    cat compile_output.txt
    rm -f compile_output.txt
    exit 1
fi
rm -f compile_output.txt

cd ..
echo "✅ 合约编译成功"
echo ""

# 读取合约字节码
IMPL_BYTECODE="0x$(cat contracts/TokenManagerV1.bin)"
PROXY_BYTECODE="0x$(cat contracts/TokenManagerProxy.bin)"

# 验证字节码
echo "🔍 验证编译后的字节码..."
if [ ${#IMPL_BYTECODE} -le 1000 ]; then
    echo "❌ 错误：实现合约字节码过短 (${#IMPL_BYTECODE} 字符，预期 >1000)"
    exit 1
fi

if [ ${#PROXY_BYTECODE} -le 1000 ]; then
    echo "❌ 错误：代理合约字节码过短 (${#PROXY_BYTECODE} 字符，预期 >1000)"
    exit 1
fi

echo "📦 字节码准备就绪:"
echo "  实现合约: ${#IMPL_BYTECODE} 字符"
echo "  代理合约: ${#PROXY_BYTECODE} 字符"
echo ""

# 部署实现合约
echo "📋 步骤 1: 部署实现合约..."
echo "  合约: TokenManagerV1"
echo "  方法: 标准 CREATE 部署"

IMPL_TX=$(cast send --private-key "$PRIVATE_KEY" --rpc-url "$RPC_URL" --gas-price "$GAS_PRICE" --gas-limit "$GAS_LIMIT" --legacy --create "$IMPL_BYTECODE" --json | jq -r '.transactionHash')

if [ $? -ne 0 ] || [ -z "$IMPL_TX" ]; then
    echo "❌ 错误：实现合约部署交易失败"
    exit 1
fi

echo "  交易哈希: $IMPL_TX"

# 等待交易确认
echo "  等待确认..."
waited=0
while [ $waited -lt $MAX_WAIT_SECONDS ]; do
    IMPL_ADDRESS=$(cast receipt "$IMPL_TX" contractAddress --rpc-url "$RPC_URL" 2>/dev/null)
    if [ -n "$IMPL_ADDRESS" ] && [ "$IMPL_ADDRESS" != "null" ]; then
        break
    fi
    sleep 1
    waited=$((waited + 1))
done

if [ -z "$IMPL_ADDRESS" ] || [ "$IMPL_ADDRESS" = "null" ]; then
    echo "❌ 错误：无法获取实现合约地址，等待 $MAX_WAIT_SECONDS 秒后失败"
    exit 1
fi

# 验证实现合约部署
IMPL_CODE=$(cast code "$IMPL_ADDRESS" --rpc-url "$RPC_URL")
if [ ${#IMPL_CODE} -le 2 ]; then
    echo "❌ 错误：实现合约未部署（地址无代码）"
    exit 1
fi

echo "✅ 实现合约部署成功，地址: $IMPL_ADDRESS"
echo ""

# 部署代理合约
echo "📋 步骤 2: 部署代理合约..."
echo "  合约: TokenManagerProxy"
echo "  实现合约: $IMPL_ADDRESS"
echo "  管理员: $PROXY_ADMIN"

PROXY_CONSTRUCTOR=$(cast abi-encode "constructor(address,address,bytes)" "$IMPL_ADDRESS" "$PROXY_ADMIN" "0x")
PROXY_DEPLOY_DATA="${PROXY_BYTECODE}${PROXY_CONSTRUCTOR:2}"

PROXY_TX=$(cast send --private-key "$PRIVATE_KEY" --rpc-url "$RPC_URL" --gas-price "$GAS_PRICE" --gas-limit "$GAS_LIMIT" --legacy --create "$PROXY_DEPLOY_DATA" --json | jq -r '.transactionHash')

if [ $? -ne 0 ] || [ -z "$PROXY_TX" ]; then
    echo "❌ 错误：代理合约部署交易失败"
    exit 1
fi

echo "  交易哈希: $PROXY_TX"

# 等待交易确认
echo "  等待确认..."
waited=0
while [ $waited -lt $MAX_WAIT_SECONDS ]; do
    PROXY_ADDRESS=$(cast receipt "$PROXY_TX" contractAddress --rpc-url "$RPC_URL" 2>/dev/null)
    if [ -n "$PROXY_ADDRESS" ] && [ "$PROXY_ADDRESS" != "null" ]; then
        break
    fi
    sleep 1
    waited=$((waited + 1))
done

if [ -z "$PROXY_ADDRESS" ] || [ "$PROXY_ADDRESS" = "null" ]; then
    echo "❌ 错误：无法获取代理合约地址，等待 $MAX_WAIT_SECONDS 秒后失败"
    exit 1
fi

# 验证代理合约部署
PROXY_CODE=$(cast code "$PROXY_ADDRESS" --rpc-url "$RPC_URL")
if [ ${#PROXY_CODE} -le 2 ]; then
    echo "❌ 错误：代理合约未部署（地址无代码）"
    exit 1
fi

echo "✅ 代理合约部署成功，地址: $PROXY_ADDRESS"
echo ""

# 初始化代理合约
echo "📋 步骤 3: 初始化合约..."
echo "  调用 initialize()，设置所有者: $OWNER_ADMIN"

INIT_DATA=$(cast calldata "initialize(address)" "$OWNER_ADMIN")
INIT_TX=$(cast send "$PROXY_ADDRESS" "$INIT_DATA" --private-key "$PRIVATE_KEY" --rpc-url "$RPC_URL" --gas-price "$GAS_PRICE" --gas-limit "$GAS_LIMIT" --legacy --json | jq -r '.transactionHash')

if [ $? -ne 0 ] || [ -z "$INIT_TX" ]; then
    echo "❌ 错误：初始化交易失败"
    exit 1
fi

echo "  交易哈希: $INIT_TX"

# 等待初始化交易确认
echo "  等待确认..."
waited=0
while [ $waited -lt $MAX_WAIT_SECONDS ]; do
    if cast receipt "$INIT_TX" status --rpc-url "$RPC_URL" 2>/dev/null | grep -q "1"; then
        break
    fi
    sleep 1
    waited=$((waited + 1))
done

if ! cast receipt "$INIT_TX" status --rpc-url "$RPC_URL" 2>/dev/null | grep -q "1"; then
    echo "❌ 错误：初始化交易失败或未确认，等待 $MAX_WAIT_SECONDS 秒后失败"
    exit 1
fi

echo "✅ 合约初始化成功"
echo ""

# 设置激活高度
echo "📋 步骤 4: 设置激活高度..."
echo "  设置激活高度为: $ACTIVATION_BLOCK"

ACTIVATION_DATA=$(cast calldata "setActivationBlock(uint256)" "$ACTIVATION_BLOCK")
ACTIVATION_TX=$(cast send "$PROXY_ADDRESS" "$ACTIVATION_DATA" --private-key "$PRIVATE_KEY" --rpc-url "$RPC_URL" --gas-price "$GAS_PRICE" --gas-limit "$GAS_LIMIT" --legacy --json | jq -r '.transactionHash')

if [ $? -ne 0 ] || [ -z "$ACTIVATION_TX" ]; then
    echo "❌ 错误：设置激活高度失败"
    exit 1
fi

echo "  交易哈希: $ACTIVATION_TX"

# 等待激活高度设置交易确认
echo "  等待确认..."
waited=0
while [ $waited -lt $MAX_WAIT_SECONDS ]; do
    if cast receipt "$ACTIVATION_TX" status --rpc-url "$RPC_URL" 2>/dev/null | grep -q "1"; then
        break
    fi
    sleep 1
    waited=$((waited + 1))
done

if ! cast receipt "$ACTIVATION_TX" status --rpc-url "$RPC_URL" 2>/dev/null | grep -q "1"; then
    echo "❌ 错误：设置激活高度交易失败或未确认，等待 $MAX_WAIT_SECONDS 秒后失败"
    exit 1
fi

echo "✅ 激活高度设置成功"
echo ""

# 验证部署
echo "🔍 步骤 5: 验证部署..."

CURRENT_OWNER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()" 2>/dev/null)
# 移除前导的24个零字节（48个字符）
CURRENT_OWNER="0x${CURRENT_OWNER:26}"
CURRENT_OWNER=$(cast to-check-sum-address "$CURRENT_OWNER" 2>/dev/null || echo "$CURRENT_OWNER")
EXPECTED_OWNER=$(cast to-check-sum-address "$OWNER_ADMIN")

if [ "$CURRENT_OWNER" != "$EXPECTED_OWNER" ]; then
    echo "❌ 错误：所有者验证失败"
    echo "  预期: $EXPECTED_OWNER"
    echo "  实际: $CURRENT_OWNER"
    exit 1
fi

VERSION_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "VERSION()" 2>/dev/null)
VERSION=$(cast to-ascii "$VERSION_RAW" 2>/dev/null || echo "无法解码")

IS_ACTIVE_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "isActive()" 2>/dev/null)
IS_ACTIVE=$([ "$IS_ACTIVE_RAW" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "已激活" || echo "未激活")

echo "✅ 验证完成:"
echo "  所有者: $CURRENT_OWNER"
echo "  版本: $VERSION"
echo "  状态: $IS_ACTIVE"
echo ""

# 部署总结
echo "🎉 部署总结"
echo "===================="
echo ""
echo "📋 已部署合约:"
echo "  实现合约: $IMPL_ADDRESS"
echo "  代理合约 (主合约): $PROXY_ADDRESS"
echo ""
echo "👑 访问控制:"
echo "  合约所有者: $CURRENT_OWNER"
echo "  代理管理员: $PROXY_ADMIN"
echo ""
echo "⚙️ 配置:"
echo "  状态: $IS_ACTIVE"
echo "  版本: $VERSION"
echo ""
echo "📝 下一步:"
echo "  1. 更新 core/vm/contracts_mint_burn.go 中的 CONFIG_CONTRACT_MANAGER_ADDRESS:"
echo "     CONFIG_CONTRACT_MANAGER_ADDRESS = common.HexToAddress(\"$PROXY_ADDRESS\")"
echo ""
echo "  2. 重新编译并重启节点"
echo ""
echo "  3. 如需激活 Token Manager:"
echo "     cast send --private-key \$ADMIN_KEY --rpc-url \"$RPC_URL\" --legacy \\"
echo "       --to \"$PROXY_ADDRESS\" \"setActivationBlock(uint256)\" 0"
echo ""
echo "  4. 如需添加烧毁白名单地址:"
echo "     cast send --private-key \$ADMIN_KEY --rpc-url \"$RPC_URL\" --legacy \\"
echo "       --to \"$PROXY_ADDRESS\" \"addBurnWhitelist(address)\" \$TARGET_ADDRESS"
echo ""
echo "✅ Token Manager 部署成功！"