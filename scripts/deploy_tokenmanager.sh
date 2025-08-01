#!/bin/bash

# Token Manager Contracts Deployment Script
set -e

echo "🚀 Token Manager Deployment Script"
echo "=================================="

# 配置参数
PRIVATE_KEY="${PRIVATE_KEY:-0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9}"
RPC_URL="${RPC_URL:-http://localhost:8123}"
GAS_PRICE="${GAS_PRICE:-1000000000}"
GAS_LIMIT="${GAS_LIMIT:-5000000}"
MAX_WAIT_SECONDS=60

# 权限分离配置
PROXY_ADMIN="${PROXY_ADMIN:-0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15}"  # 代理管理员，控制TokenManager合约的升级 (upgrade)
OWNER_ADDRESS="${OWNER_ADDRESS:-$PROXY_ADMIN}"  # TokenManager合约的Owner，控制合约启停 (pause/unpause/setActivationBlock)，暂时让其 = ProxyAdmin
ADMIN_ADDRESS="${ADMIN_ADDRESS:-0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534}"  # 业务Admin，角色(minter/burner)管理和mint白名单管理

# 激活配置
ACTIVATION_BLOCK="${ACTIVATION_BLOCK:-0}"

# 验证环境变量
if [[ "$PRIVATE_KEY" == "0xYOUR_PRIVATE_KEY" || -z "$PRIVATE_KEY" ]]; then
    echo "❌ 错误：PRIVATE_KEY 未设置或无效"
    exit 1
fi

# 检查网络连接
if ! cast chain-id --rpc-url "$RPC_URL" > /dev/null; then
    echo "❌ 错误：无法连接到 RPC 端点 $RPC_URL"
    exit 1
fi

# 检查部署者余额
DEPLOYER=$(cast wallet address --private-key "$PRIVATE_KEY")
BALANCE=$(cast balance "$DEPLOYER" --rpc-url "$RPC_URL" --ether)
if (( $(echo "$BALANCE < 0.01" | bc -l) )); then
    echo "❌ 错误：部署者余额不足 ($BALANCE ETH，至少需要 0.01 ETH)"
    exit 1
fi

# 编译合约
cd ../contracts || exit 1
if ! command -v solc &> /dev/null; then
    echo "❌ 错误：未找到 solc 编译器"
    exit 1
fi

if ! solc --bin --evm-version paris TokenManagerV1.sol -o . --overwrite --base-path . --include-path node_modules/ > /dev/null 2>&1; then
    echo "❌ 错误：编译 TokenManagerV1.sol 失败"
    exit 1
fi

if ! solc --bin --evm-version paris TokenManagerProxy.sol -o . --overwrite --base-path . --include-path node_modules/ > /dev/null 2>&1; then
    echo "❌ 错误：编译 TokenManagerProxy.sol 失败"
    exit 1
fi

cd ..

# 读取合约字节码
IMPL_BYTECODE="0x$(cat contracts/TokenManagerV1.bin)"
PROXY_BYTECODE="0x$(cat contracts/TokenManagerProxy.bin)"

# 部署实现合约
echo "📋 部署实现合约..."
IMPL_TX=$(cast send --private-key "$PRIVATE_KEY" --rpc-url "$RPC_URL" --gas-price "$GAS_PRICE" --gas-limit "$GAS_LIMIT" --legacy --create "$IMPL_BYTECODE" --json | jq -r '.transactionHash')
if [ $? -ne 0 ] || [ -z "$IMPL_TX" ]; then
    echo "❌ 错误：实现合约部署失败"
    exit 1
fi

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
    echo "❌ 错误：实现合约部署失败"
    exit 1
fi

echo "✅ 实现合约: $IMPL_ADDRESS"

# 部署代理合约
echo "📋 部署代理合约..."
PROXY_CONSTRUCTOR=$(cast abi-encode "constructor(address,address,bytes)" "$IMPL_ADDRESS" "$PROXY_ADMIN" "0x")
PROXY_DEPLOY_DATA="${PROXY_BYTECODE}${PROXY_CONSTRUCTOR:2}"

PROXY_TX=$(cast send --private-key "$PRIVATE_KEY" --rpc-url "$RPC_URL" --gas-price "$GAS_PRICE" --gas-limit "$GAS_LIMIT" --legacy --create "$PROXY_DEPLOY_DATA" --json | jq -r '.transactionHash')
if [ $? -ne 0 ] || [ -z "$PROXY_TX" ]; then
    echo "❌ 错误：代理合约部署失败"
    exit 1
fi

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
    echo "❌ 错误：代理合约部署失败"
    exit 1
fi

echo "✅ 代理合约: $PROXY_ADDRESS"

# 初始化合约
echo "📋 初始化合约..."
INIT_DATA=$(cast calldata "initialize(address,address)" "$OWNER_ADDRESS" "$ADMIN_ADDRESS")
INIT_TX=$(cast send "$PROXY_ADDRESS" "$INIT_DATA" --private-key "$PRIVATE_KEY" --rpc-url "$RPC_URL" --gas-price "$GAS_PRICE" --gas-limit "$GAS_LIMIT" --legacy --json | jq -r '.transactionHash')
if [ $? -ne 0 ] || [ -z "$INIT_TX" ]; then
    echo "❌ 错误：初始化失败"
    exit 1
fi

# 设置激活高度
echo "📋 设置激活高度..."
ACTIVATION_DATA=$(cast calldata "setActivationBlock(uint256)" "$ACTIVATION_BLOCK")
ACTIVATION_TX=$(cast send "$PROXY_ADDRESS" "$ACTIVATION_DATA" --private-key "$PRIVATE_KEY" --rpc-url "$RPC_URL" --gas-price "$GAS_PRICE" --gas-limit "$GAS_LIMIT" --legacy --json | jq -r '.transactionHash')
if [ $? -ne 0 ] || [ -z "$ACTIVATION_TX" ]; then
    echo "❌ 错误：设置激活高度失败"
    exit 1
fi

echo "✅ 激活高度设置成功"

# 验证部署
echo "📋 验证部署..."

# 验证Owner
CURRENT_OWNER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()" 2>/dev/null)
# 移除前导的24个零字节（48个字符）
CURRENT_OWNER="0x${CURRENT_OWNER:26}"
CURRENT_OWNER=$(cast to-check-sum-address "$CURRENT_OWNER" 2>/dev/null || echo "$CURRENT_OWNER")
EXPECTED_OWNER=$(cast to-check-sum-address "$OWNER_ADDRESS")

if [ "$CURRENT_OWNER" != "$EXPECTED_OWNER" ]; then
    echo "❌ 错误：Owner验证失败"
    echo "  预期: $EXPECTED_OWNER"
    echo "  实际: $CURRENT_OWNER"
    exit 1
fi

# 验证Admin
CURRENT_ADMIN=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getAdmin()" 2>/dev/null)
# 移除前导的24个零字节（48个字符）
CURRENT_ADMIN="0x${CURRENT_ADMIN:26}"
CURRENT_ADMIN=$(cast to-check-sum-address "$CURRENT_ADMIN" 2>/dev/null || echo "$CURRENT_ADMIN")
EXPECTED_ADMIN=$(cast to-check-sum-address "$ADMIN_ADDRESS")

if [ "$CURRENT_ADMIN" != "$EXPECTED_ADMIN" ]; then
    echo "❌ 错误：Admin验证失败"
    echo "  预期: $EXPECTED_ADMIN"
    echo "  实际: $CURRENT_ADMIN"
    exit 1
fi

VERSION_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "VERSION()" 2>/dev/null)
VERSION=$(cast to-ascii "$VERSION_RAW" 2>/dev/null || echo "无法解码")

IS_ACTIVE_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "isActive()" 2>/dev/null)
IS_ACTIVE=$([ "$IS_ACTIVE_RAW" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "已激活" || echo "未激活")

# 部署总结
echo "🎉 部署完成"
echo "===================="
echo ""
echo "📋 已部署合约:"
echo "  实现合约: $IMPL_ADDRESS"
echo "  代理合约 (主合约): $PROXY_ADDRESS"
echo ""
echo "👑 权限分离架构:"
echo "  代理管理员: $PROXY_ADMIN (合约升级) = Owner地址"
echo "  系统Owner: $CURRENT_OWNER (pause/unpause)"
echo "  业务Admin: $CURRENT_ADMIN (operator管理/mint/cleanUp)"
echo ""
echo "⚙️ 配置:"
echo "  状态: $IS_ACTIVE"
echo "  版本: $VERSION"
echo ""