#!/bin/bash
set -e

echo "🔬 Token Manager 测试脚本"
echo "=========================="

# 配置参数
RPC_URL="${RPC_URL:-http://localhost:8123}"
PROXY_ADDRESS="${PROXY_ADDRESS:-0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab}"
TARGET_ADDRESS="0x000000000000000000000000000000000000dEaD"

# 测试账户
ADMIN_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
ADMIN="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"

OWNER_PRIVATE_KEY="0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9"
OWNER="0xDE282DC882bbB5100b8A24E30D38a2D5B3080c15"

# 测试操作员账户
OPERATOR_PRIVATE_KEY="0x3c9229289a6125f7fdf1885a77bb12c37a8d3b4962d936f7e3084dece32a3ca1"
OPERATOR=$(cast wallet address --private-key "$OPERATOR_PRIVATE_KEY")  # 从私钥计算正确地址

# 新的测试账户
NEW_OPERATOR_KEY="0x4bbbf85ce3377467afe5d46f804f221813b2bb87f24d81f60f1fcdbf7cbf4356"
NEW_OPERATOR="0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
NEW_ADMIN_PRIVATE_KEY="0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
NEW_ADMIN="0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
NEW_OWNER_PRIVATE_KEY="0x689af8efa8c651a91ad287602527f3af2fe9f6501a7ac4b061667b5a93e037fd"
NEW_OWNER="0xbDA5747bFD65F08deb54cb465eB87D40e51B197E"

# 测试金额
MINT_AMOUNT="1000000000000000000"  # 1 ETH

echo "🎯 测试配置:"
echo "  代理合约: $PROXY_ADDRESS"
echo "  目标地址: $TARGET_ADDRESS"
echo "  管理员: $ADMIN"
echo "  所有者: $OWNER"
echo "  操作员: $OPERATOR"
echo ""

# 状态重置：确保admin和operator处于期望状态
echo "🔄 重置合约状态..."
echo "----------------------------------------"

# 检查并重置Admin（如果需要）
CURRENT_ADMIN_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "admin()" 2>/dev/null)
CURRENT_ADMIN_ADDR="0x${CURRENT_ADMIN_RESULT:26}"
CURRENT_ADMIN_ADDR=$(cast to-check-sum-address "$CURRENT_ADMIN_ADDR")
EXPECTED_ADMIN=$(cast to-check-sum-address "$ADMIN")

if [ "$CURRENT_ADMIN_ADDR" != "$EXPECTED_ADMIN" ]; then
    echo "❌ Admin地址不匹配，需要重新部署合约"
    echo "  当前: $CURRENT_ADMIN_ADDR"
    echo "  期望: $EXPECTED_ADMIN"
    exit 1
fi
echo "✅ Admin状态正确: $CURRENT_ADMIN_ADDR"

# 检查并重置Operator到期望地址（如果需要）
OPERATOR_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()")
CURRENT_OPERATOR="0x${OPERATOR_RESULT:26}"
CURRENT_OPERATOR=$(cast to-check-sum-address "$CURRENT_OPERATOR")
OPERATOR_CHECKSUM=$(cast to-check-sum-address "$OPERATOR")

if [ "$CURRENT_OPERATOR" != "$OPERATOR_CHECKSUM" ]; then
    echo "ℹ️  重置Operator到期望地址..."
    cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "setOperator(address)" "$OPERATOR" >/dev/null 2>&1
    echo "✅ Operator重置成功: $OPERATOR_CHECKSUM"
else
    echo "✅ Operator已是期望地址: $OPERATOR_CHECKSUM"
fi
echo ""

# 初始化测试账户资金
echo "💰 初始化测试账户..."
TEST_ACCOUNTS=("$OPERATOR" "$NEW_OPERATOR" "$NEW_ADMIN" "$NEW_OWNER")
TRANSFER_AMOUNT_WEI=100000000000000000  # 0.1 ETH

for ACCOUNT in "${TEST_ACCOUNTS[@]}"; do
    BALANCE=$(cast balance "$ACCOUNT" --rpc-url "$RPC_URL" --ether)
    if (( $(echo "$BALANCE < 0.1" | bc -l) )); then
        echo "  为账户 $ACCOUNT 转账 0.1 ETH..."
        cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" \
            --gas-limit 30000 --legacy \
            --value "$TRANSFER_AMOUNT_WEI" "$ACCOUNT" >/dev/null 2>&1
    fi
done
echo "✅ 测试账户初始化完成"
echo ""

# 步骤1: Operator管理测试
echo "🔬 步骤 1: Operator管理测试"
echo "----------------------------------------"

# 验证当前Admin
echo "ℹ️  验证当前Admin..."
CURRENT_ADMIN_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "admin()" 2>/dev/null)
CURRENT_ADMIN_ADDR="0x${CURRENT_ADMIN_RESULT:26}"
CURRENT_ADMIN_ADDR=$(cast to-check-sum-address "$CURRENT_ADMIN_ADDR")
EXPECTED_ADMIN=$(cast to-check-sum-address "$ADMIN")

if [ "$CURRENT_ADMIN_ADDR" != "$EXPECTED_ADMIN" ]; then
    echo "❌ 错误：当前Admin ($CURRENT_ADMIN_ADDR) 与脚本配置 ($EXPECTED_ADMIN) 不匹配"
    echo "请检查ADMIN_PRIVATE_KEY是否正确或合约admin配置"
    exit 1
fi
echo "  当前Admin验证通过: $CURRENT_ADMIN_ADDR"

echo "ℹ️  清除现有Operator..."
# 检查当前operator状态，只有非零地址才需要清除
CURRENT_OP_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()" 2>/dev/null)
CURRENT_OP="0x${CURRENT_OP_RESULT:26}"
if [ "$CURRENT_OP" != "0x0000000000000000000000000000000000000000" ]; then
    cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "setOperator(address)" "0x0000000000000000000000000000000000000000" >/dev/null 2>&1
    echo "  已清除现有Operator: $CURRENT_OP"
else
    echo "  当前Operator已为零地址，无需清除"
fi

echo "ℹ️  设置Operator..."
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "$OPERATOR" >/dev/null 2>&1

OPERATOR_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()")
CURRENT_OPERATOR="0x${OPERATOR_RESULT:26}"
CURRENT_OPERATOR=$(cast to-check-sum-address "$CURRENT_OPERATOR")
OPERATOR_CHECKSUM=$(cast to-check-sum-address "$OPERATOR")
if [ "$CURRENT_OPERATOR" = "$OPERATOR_CHECKSUM" ]; then
    echo "✅ Operator设置成功"
else
    echo "❌ Operator设置失败"
    echo "  预期: $OPERATOR_CHECKSUM"
    echo "  实际: $CURRENT_OPERATOR"
    exit 1
fi

echo "ℹ️  测试新Operator设置..."
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "$NEW_OPERATOR" >/dev/null 2>&1

NEW_OPERATOR_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()")
CURRENT_NEW_OPERATOR="0x${NEW_OPERATOR_RESULT:26}"
CURRENT_NEW_OPERATOR=$(cast to-check-sum-address "$CURRENT_NEW_OPERATOR")
NEW_OPERATOR_CHECKSUM=$(cast to-check-sum-address "$NEW_OPERATOR")
if [ "$CURRENT_NEW_OPERATOR" = "$NEW_OPERATOR_CHECKSUM" ]; then
    echo "✅ 新Operator设置成功"
else
    echo "❌ 新Operator设置失败"
    exit 1
fi

# 测试移除Operator功能
echo "ℹ️  测试移除Operator (设置为零地址)..."
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "0x0000000000000000000000000000000000000000" >/dev/null 2>&1

ZERO_OP_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()")
ZERO_OP="0x${ZERO_OP_RESULT:26}"
if [ "$ZERO_OP" = "0x0000000000000000000000000000000000000000" ]; then
    echo "✅ Operator移除成功"
else
    echo "❌ Operator移除失败"
    exit 1
fi

# 恢复原来的Operator
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "$OPERATOR" >/dev/null 2>&1
echo ""

# 步骤2: Mint操作测试  
echo "🔬 步骤 2: Mint操作测试"
echo "----------------------------------------"

# 预检查：验证合约状态
echo "ℹ️  检查合约状态..."
IS_ACTIVE_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "isActive()" 2>/dev/null)
IS_ACTIVE=$([ "$IS_ACTIVE_RAW" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "true" || echo "false")
echo "  合约激活状态: $IS_ACTIVE"

PAUSED_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "paused()" 2>/dev/null)
PAUSED=$([ "$PAUSED_RAW" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "true" || echo "false")
echo "  合约暂停状态: $PAUSED"

CURRENT_OP_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()" 2>/dev/null)
CURRENT_OP_ADDR="0x${CURRENT_OP_RAW:26}"
CURRENT_OP_ADDR=$(cast to-check-sum-address "$CURRENT_OP_ADDR")
EXPECTED_OP=$(cast to-check-sum-address "$OPERATOR")
echo "  当前Operator: $CURRENT_OP_ADDR"

if [ "$CURRENT_OP_ADDR" != "$EXPECTED_OP" ]; then
    echo "❌ 错误：当前Operator与预期不匹配"
    echo "  预期: $EXPECTED_OP"
    echo "  实际: $CURRENT_OP_ADDR"
    exit 1
fi

OPERATOR_BALANCE_BEFORE=$(cast balance "$OPERATOR" --rpc-url "$RPC_URL")
echo "ℹ️  Operator余额 (操作前): $OPERATOR_BALANCE_BEFORE wei"

echo "ℹ️  执行mint操作 (金额: $MINT_AMOUNT wei)..."
set +e  # 临时禁用严格模式
MINT_RESULT=$(cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "mint(uint256)" "$MINT_AMOUNT" 2>&1)
MINT_EXIT_CODE=$?
set -e  # 重新启用严格模式

if [ $MINT_EXIT_CODE -ne 0 ]; then
    echo "❌ Mint操作失败:"
    echo "$MINT_RESULT"
    exit 1
fi

OPERATOR_BALANCE_AFTER=$(cast balance "$OPERATOR" --rpc-url "$RPC_URL")
echo "ℹ️  Operator余额 (操作后): $OPERATOR_BALANCE_AFTER wei"

BALANCE_DIFF=$((OPERATOR_BALANCE_AFTER - OPERATOR_BALANCE_BEFORE))
echo "ℹ️  余额变化: $BALANCE_DIFF wei"

# 考虑gas费用，实际增加应该接近mint金额（允许一定误差）
MIN_EXPECTED=$((MINT_AMOUNT - 100000000000000000))  # 允许0.1 ETH的gas费用误差
if [ "$BALANCE_DIFF" -ge "$MIN_EXPECTED" ]; then
    echo "✅ Mint操作成功"
else
    echo "❌ Mint操作失败 (余额变化: $BALANCE_DIFF wei，预期至少: $MIN_EXPECTED wei)"
    exit 1
fi
echo ""

# 步骤3: Cleanup操作测试
echo "🔬 步骤 3: Cleanup操作测试"
echo "----------------------------------------"

TARGET_BALANCE=$(cast balance "$TARGET_ADDRESS" --rpc-url "$RPC_URL")

if [ "$TARGET_BALANCE" -le 1 ]; then
    echo "ℹ️  目标地址余额不足，用Admin转入 2 ETH..."
    cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        --value 2000000000000000000 "$TARGET_ADDRESS" >/dev/null 2>&1
    TARGET_BALANCE=$(cast balance "$TARGET_ADDRESS" --rpc-url "$RPC_URL")
fi

echo "ℹ️  目标地址余额 (清理前): $TARGET_BALANCE wei"

set +e  # 临时禁用严格模式
CLEANUP_RESULT=$(cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "cleanup()" 2>&1)
CLEANUP_EXIT_CODE=$?
set -e  # 重新启用严格模式

if [ $CLEANUP_EXIT_CODE -ne 0 ]; then
    echo "❌ Cleanup操作失败:"
    echo "$CLEANUP_RESULT"
    exit 1
fi

TARGET_BALANCE_AFTER=$(cast balance "$TARGET_ADDRESS" --rpc-url "$RPC_URL")
echo "ℹ️  目标地址余额 (清理后): $TARGET_BALANCE_AFTER wei"

if [ "$TARGET_BALANCE_AFTER" -eq 1 ]; then
    echo "✅ Cleanup操作成功 (保留1 wei)"
else
    echo "❌ Cleanup操作失败 (余额: $TARGET_BALANCE_AFTER wei)"
    exit 1
fi

# 测试对已清理地址的cleanup操作（应该成功，幂等性）
set +e  # 临时禁用严格模式
CLEANUP2_RESULT=$(cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "cleanup()" 2>&1)
CLEANUP2_EXIT_CODE=$?
set -e  # 重新启用严格模式

if [ $CLEANUP2_EXIT_CODE -ne 0 ]; then
    echo "❌ 第二次Cleanup操作失败:"
    echo "$CLEANUP2_RESULT"
    exit 1
fi

FINAL_BALANCE=$(cast balance "$TARGET_ADDRESS" --rpc-url "$RPC_URL")
if [ "$FINAL_BALANCE" -eq 1 ]; then
    echo "✅ 对已清理地址的cleanup操作成功 (幂等性)"
else
    echo "❌ 对已清理地址的cleanup操作失败"
    exit 1
fi
echo ""

# 步骤4: 暂停/恢复功能测试
echo "🔬 步骤 4: 暂停/恢复功能测试"
echo "----------------------------------------"

echo "ℹ️  暂停合约..."
cast send --private-key "$OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "pause()" >/dev/null 2>&1

PAUSED_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "paused()")
if [ "$PAUSED_RESULT" = "0x0000000000000000000000000000000000000000000000000000000000000001" ]; then
    echo "✅ 合约暂停成功"
else
    echo "❌ 合约暂停失败"
    exit 1
fi

echo "ℹ️  测试暂停状态下的mint操作（应该失败）..."
set +e
MINT_RESULT=$(cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "mint(uint256)" "$MINT_AMOUNT" 2>&1)
set -e

if echo "$MINT_RESULT" | grep -q "revert\|failed"; then
    echo "✅ 暂停状态下mint操作被正确拒绝"
else
    echo "❌ 暂停状态下mint操作未被拒绝"
    exit 1
fi

echo "ℹ️  恢复合约..."
cast send --private-key "$OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "unpause()" >/dev/null 2>&1

PAUSED_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "paused()")
if [ "$PAUSED_RESULT" = "0x0000000000000000000000000000000000000000000000000000000000000000" ]; then
    echo "✅ 合约恢复成功"
else
    echo "❌ 合约恢复失败"
    exit 1
fi
echo ""

# 步骤5: 查询功能测试
echo "🔬 步骤 5: 查询功能测试"
echo "----------------------------------------"

# 测试operator查询
CURRENT_OP_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()")
CURRENT_OP="0x${CURRENT_OP_RESULT:26}"
CURRENT_OP=$(cast to-check-sum-address "$CURRENT_OP")
if [ "$CURRENT_OP" = "$(cast to-check-sum-address "$OPERATOR")" ]; then
    echo "✅ operator查询正常"
else
    echo "❌ operator查询异常"
    exit 1
fi

# 测试admin查询
ADMIN_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "admin()")
ADMIN_ADDR="0x${ADMIN_RESULT:26}"
ADMIN_ADDR=$(cast to-check-sum-address "$ADMIN_ADDR")
if [ "$ADMIN_ADDR" = "$(cast to-check-sum-address "$ADMIN")" ]; then
    echo "✅ admin查询正常"
else
    echo "❌ admin查询异常"
    exit 1
fi



# 测试VERSION
VERSION_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "VERSION()")
VERSION=$(cast to-ascii "$VERSION_RAW" 2>/dev/null || echo "无法解码")
# 去除前后空格
VERSION=$(echo "$VERSION" | xargs)
if [ "$VERSION" = "1.0.0" ]; then
    echo "✅ VERSION查询正常"
else
    echo "❌ VERSION查询异常: '$VERSION'"
    exit 1
fi

# 测试isActive
IS_ACTIVE_RAW=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "isActive()")
IS_ACTIVE=$([ "$IS_ACTIVE_RAW" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "已激活" || echo "未激活")
echo "ℹ️  合约状态: $IS_ACTIVE"
echo ""

# 步骤6: Admin角色转移测试
echo "🔬 步骤 6: Admin角色转移测试"
echo "----------------------------------------"

echo "ℹ️  转移Admin角色..."
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setAdmin(address)" "$NEW_ADMIN" >/dev/null 2>&1

# 验证Admin转移
NEW_ADMIN_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "admin()")
NEW_ADMIN_ADDR="0x${NEW_ADMIN_RESULT:26}"
NEW_ADMIN_ADDR=$(cast to-check-sum-address "$NEW_ADMIN_ADDR")
if [ "$NEW_ADMIN_ADDR" = "$(cast to-check-sum-address "$NEW_ADMIN")" ]; then
    echo "✅ Admin角色转移成功"
else
    echo "❌ Admin角色转移失败"
    exit 1
fi

# 验证权限转移：测试旧admin失去权限，新admin获得权限
CURRENT_OP_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()")
CURRENT_OP="0x${CURRENT_OP_RESULT:26}"
CURRENT_OP=$(cast to-check-sum-address "$CURRENT_OP")

# 选择一个不同的测试地址（确保不是当前operator）
if [ "$CURRENT_OP" = "$(cast to-check-sum-address "$OWNER")" ]; then
    TEST_OP_ADDRESS="$NEW_OWNER"
else
    TEST_OP_ADDRESS="$OWNER"
fi

# 测试旧admin权限失效
set +e
OLD_ADMIN_RESULT=$(cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "$TEST_OP_ADDRESS" 2>&1)
set -e

if echo "$OLD_ADMIN_RESULT" | grep -q "revert\|failed"; then
    # 测试新admin权限生效
    if cast send --private-key "$NEW_ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "setOperator(address)" "$TEST_OP_ADDRESS" >/dev/null 2>&1; then
        echo "✅ 管理员角色转移成功"
        
        # 恢复原来的operator
        cast send --private-key "$NEW_ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
            "$PROXY_ADDRESS" "setOperator(address)" "$CURRENT_OP" >/dev/null 2>&1
    else
        echo "❌ 新管理员权限未生效"
        exit 1
    fi
else
    echo "❌ 旧管理员权限未失效"
    exit 1
fi
echo ""

# 步骤7: 所有者转移测试
echo "🔬 步骤 7: 所有者转移测试"
echo "----------------------------------------"

# 检查当前owner
CURRENT_OWNER_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()")
CURRENT_OWNER_ADDR="0x${CURRENT_OWNER_RESULT:26}"
CURRENT_OWNER_ADDR=$(cast to-check-sum-address "$CURRENT_OWNER_ADDR")

if [ "$CURRENT_OWNER_ADDR" != "$(cast to-check-sum-address "$OWNER")" ]; then
    echo "⚠️  当前owner ($CURRENT_OWNER_ADDR) 与脚本中的OWNER ($OWNER) 不匹配，跳过owner转移测试"
else
    echo "ℹ️  转移Owner权限..."
    cast send --private-key "$OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "transferOwnership(address)" "$NEW_OWNER" >/dev/null 2>&1

    # 验证Owner转移
    NEW_OWNER_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()")
    NEW_OWNER_ADDR="0x${NEW_OWNER_RESULT:26}"
    NEW_OWNER_ADDR=$(cast to-check-sum-address "$NEW_OWNER_ADDR")
    if [ "$NEW_OWNER_ADDR" = "$(cast to-check-sum-address "$NEW_OWNER")" ]; then
        echo "✅ 所有者转移成功"
    else
        echo "❌ 所有者转移失败"
        exit 1
    fi

    # 验证旧owner权限失效，新owner权限生效
    set +e
    OLD_OWNER_RESULT=$(cast send --private-key "$OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "pause()" 2>&1)
    set -e

    if echo "$OLD_OWNER_RESULT" | grep -q "revert\|failed"; then
        echo "✅ 所有者权限转移成功"
    else
        echo "❌ 旧所有者权限未失效"
        exit 1
    fi
fi
echo ""

# 步骤8: 恢复状态
echo "🔄 步骤 8: 恢复测试状态"
echo "----------------------------------------"

echo "ℹ️  恢复Admin到原始地址..."
# 使用当前的NEW_ADMIN权限恢复admin
cast send --private-key "$NEW_ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setAdmin(address)" "$ADMIN" >/dev/null 2>&1

# 验证Admin恢复
RESTORED_ADMIN_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "admin()")
RESTORED_ADMIN_ADDR="0x${RESTORED_ADMIN_RESULT:26}"
RESTORED_ADMIN_ADDR=$(cast to-check-sum-address "$RESTORED_ADMIN_ADDR")
if [ "$RESTORED_ADMIN_ADDR" = "$(cast to-check-sum-address "$ADMIN")" ]; then
    echo "✅ Admin恢复成功: $RESTORED_ADMIN_ADDR"
else
    echo "❌ Admin恢复失败"
    exit 1
fi

echo "ℹ️  恢复Owner到原始地址..."
# 使用当前的NEW_OWNER权限恢复owner
cast send --private-key "$NEW_OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "transferOwnership(address)" "$OWNER" >/dev/null 2>&1

# 验证Owner恢复
RESTORED_OWNER_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()")
RESTORED_OWNER_ADDR="0x${RESTORED_OWNER_RESULT:26}"
RESTORED_OWNER_ADDR=$(cast to-check-sum-address "$RESTORED_OWNER_ADDR")
if [ "$RESTORED_OWNER_ADDR" = "$(cast to-check-sum-address "$OWNER")" ]; then
    echo "✅ Owner恢复成功: $RESTORED_OWNER_ADDR"
else
    echo "❌ Owner恢复失败"
    exit 1
fi

echo "ℹ️  恢复Operator到原始地址..."
# 检查当前operator是否需要恢复
CURRENT_OPERATOR_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "operator()")
CURRENT_OPERATOR_ADDR="0x${CURRENT_OPERATOR_RESULT:26}"
CURRENT_OPERATOR_ADDR=$(cast to-check-sum-address "$CURRENT_OPERATOR_ADDR")
EXPECTED_OPERATOR=$(cast to-check-sum-address "$OPERATOR")

if [ "$CURRENT_OPERATOR_ADDR" != "$EXPECTED_OPERATOR" ]; then
    # 使用恢复的admin权限重置operator
    cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "setOperator(address)" "$OPERATOR" >/dev/null 2>&1
    echo "✅ Operator恢复成功: $EXPECTED_OPERATOR"
else
    echo "✅ Operator已是期望地址: $EXPECTED_OPERATOR"
fi

echo "✅ 所有状态恢复完成"
echo ""

echo "🎉 所有测试完成!"
echo "  ✅ Operator管理正常"
echo "  ✅ Mint操作正常"
echo "  ✅ Cleanup操作正常"
echo "  ✅ 暂停/恢复功能正常"
echo "  ✅ 查询功能正常"
echo "  ✅ Admin转移功能正常"
echo "  ✅ 所有者转移功能正常"
echo "  ✅ 状态恢复正常"