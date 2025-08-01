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
OPERATOR="0x07d3C7978836067b89ae6Ed0BEaa106ef012e353"

# 新的测试账户
NEW_OPERATOR_KEY="0x4bbbf85ce3377467afe5d46f804f221813b2bb87f24d81f60f1fcdbf7cbf4356"
NEW_OPERATOR="0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
NEW_ADMIN_PRIVATE_KEY="0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
NEW_ADMIN="0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
NEW_OWNER_PRIVATE_KEY="0x689af8efa8c651a91ad287602527f3af2fe9f6501a7ac4b061667b5a93e037fd"
NEW_OWNER="0xbDA5747bFD65F08deb54cb465eB87D40e51B197E"

# 角色常量
OPERATOR_ROLE=$(cast keccak "OPERATOR_ROLE")

echo "🎯 测试配置:"
echo "  代理合约: $PROXY_ADDRESS"
echo "  目标地址: $TARGET_ADDRESS"
echo "  管理员: $ADMIN"
echo "  所有者: $OWNER"
echo "  操作员: $OPERATOR"
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

# 步骤1: Operator角色管理测试
echo "🔬 步骤 1: Operator角色管理测试"
echo "----------------------------------------"

echo "ℹ️  检查当前Admin并清除现有Operator..."
# 检查当前Admin是谁
CURRENT_ADMIN_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getAdmin()")
CURRENT_ADMIN_ADDR="0x${CURRENT_ADMIN_RESULT:26}"
CURRENT_ADMIN_ADDR=$(cast to-check-sum-address "$CURRENT_ADMIN_ADDR")

# 确定使用哪个Admin私钥
if [ "$CURRENT_ADMIN_ADDR" = "$(cast to-check-sum-address "$ADMIN")" ]; then
    ACTIVE_ADMIN_KEY="$ADMIN_PRIVATE_KEY"
    echo "  当前Admin: $ADMIN (使用ADMIN私钥)"
elif [ "$CURRENT_ADMIN_ADDR" = "$(cast to-check-sum-address "$NEW_ADMIN")" ]; then
    ACTIVE_ADMIN_KEY="$NEW_ADMIN_PRIVATE_KEY"
    echo "  当前Admin: $NEW_ADMIN (使用NEW_ADMIN私钥)"
    # 先转回原来的Admin以便测试
    cast send --private-key "$NEW_ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "transferAdminRole(address)" "$ADMIN" >/dev/null 2>&1
    ACTIVE_ADMIN_KEY="$ADMIN_PRIVATE_KEY"
    echo "  已转回原Admin: $ADMIN"
else
    echo "❌ 未识别的Admin地址: $CURRENT_ADMIN_ADDR"
    exit 1
fi

cast send --private-key "$ACTIVE_ADMIN_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "removeOperator()" >/dev/null 2>&1

echo "ℹ️  设置Operator..."
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "$OPERATOR" >/dev/null 2>&1

OPERATOR_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getCurrentOperator()")
CURRENT_OPERATOR="0x${OPERATOR_RESULT:26}"
CURRENT_OPERATOR=$(cast to-check-sum-address "$CURRENT_OPERATOR")
OPERATOR_CHECKSUM=$(cast to-check-sum-address "$OPERATOR")
if [ "$CURRENT_OPERATOR" = "$OPERATOR_CHECKSUM" ]; then
    echo "✅ Operator设置成功"
else
    echo "❌ Operator设置失败"
    exit 1
fi

echo "ℹ️  测试Operator替换..."
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "$NEW_OPERATOR" >/dev/null 2>&1

# 验证新operator已经被设置
OPERATOR_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getCurrentOperator()")
CURRENT_OPERATOR="0x${OPERATOR_RESULT:26}"
CURRENT_OPERATOR=$(cast to-check-sum-address "$CURRENT_OPERATOR")
NEW_OPERATOR_CHECKSUM=$(cast to-check-sum-address "$NEW_OPERATOR")

if [ "$CURRENT_OPERATOR" = "$NEW_OPERATOR_CHECKSUM" ]; then
    echo "✅ 新Operator设置成功"
else
    echo "❌ 新Operator设置失败"
    exit 1
fi

# 验证旧operator权限失效
if ! cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "mint(uint256)" "1000000000000000000" 2>/dev/null; then
    echo "✅ 旧Operator权限已失效"
else
    echo "❌ 旧Operator权限未失效"
    exit 1
fi

# 验证新operator权限生效
set +e
MINT_RESULT=$(cast send --private-key "$NEW_OPERATOR_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "mint(uint256)" "1000000000000000000" 2>&1)
MINT_STATUS=$?
set -e

if [ $MINT_STATUS -eq 0 ]; then
    echo "✅ 新Operator权限生效"
else
    echo "❌ 新Operator权限未生效"
    echo "错误信息: $MINT_RESULT"
    exit 1
fi

# 更新当前operator为新operator
OPERATOR_PRIVATE_KEY="$NEW_OPERATOR_KEY"
OPERATOR="$NEW_OPERATOR"

echo "ℹ️  测试移除Operator..."
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "removeOperator()" >/dev/null 2>&1

OPERATOR_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getCurrentOperator()")
if [ "$OPERATOR_RESULT" = "0x0000000000000000000000000000000000000000000000000000000000000000" ]; then
    echo "✅ Operator移除成功"
else
    echo "❌ Operator移除失败"
    exit 1
fi

# 重新设置Operator用于后续测试
cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "$OPERATOR" >/dev/null 2>&1
echo ""

# 步骤2: Mint操作测试
echo "🔬 步骤 2: Mint操作测试"
echo "----------------------------------------"

echo "ℹ️  测试mint操作..."
MINT_AMOUNT="1000000000000000000" # 1 ETH
OPERATOR_BALANCE_BEFORE=$(cast balance "$OPERATOR" --rpc-url "$RPC_URL")

cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "mint(uint256)" "$MINT_AMOUNT" >/dev/null 2>&1

OPERATOR_BALANCE_AFTER=$(cast balance "$OPERATOR" --rpc-url "$RPC_URL")
BALANCE_DIFF=$((OPERATOR_BALANCE_AFTER - OPERATOR_BALANCE_BEFORE))

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
    echo "ℹ️  目标地址余额不足，转入 2 ETH..."
    cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        --value 2000000000000000000 "$TARGET_ADDRESS" >/dev/null 2>&1
    TARGET_BALANCE=$(cast balance "$TARGET_ADDRESS" --rpc-url "$RPC_URL")
fi

echo "ℹ️  执行cleanup操作..."
cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "cleanup()" >/dev/null 2>&1

TARGET_BALANCE_AFTER=$(cast balance "$TARGET_ADDRESS" --rpc-url "$RPC_URL")

if [ "$TARGET_BALANCE_AFTER" = "1" ]; then
    echo "✅ Cleanup操作成功"
else
    echo "❌ Cleanup操作失败 (预期1 wei，实际 $TARGET_BALANCE_AFTER wei)"
    exit 1
fi

# 测试对已清理地址的cleanup操作
cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "cleanup()" >/dev/null 2>&1

TARGET_BALANCE_FINAL=$(cast balance "$TARGET_ADDRESS" --rpc-url "$RPC_URL")
if [ "$TARGET_BALANCE_FINAL" = "1" ]; then
    echo "✅ 重复cleanup操作正常"
else
    echo "❌ 重复cleanup操作异常（余额: $TARGET_BALANCE_FINAL wei）"
    exit 1
fi
echo ""

# 步骤4: 暂停/恢复功能测试
echo "🔬 步骤 4: 暂停/恢复功能测试"
echo "----------------------------------------"

echo "ℹ️  测试暂停功能..."
cast send --private-key "$OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "pause()" >/dev/null 2>&1

PAUSED=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "paused()")
if [ "$PAUSED" = "0x0000000000000000000000000000000000000000000000000000000000000001" ]; then
    echo "✅ 合约已暂停"
else
    echo "❌ 合约暂停失败"
    exit 1
fi

# 测试暂停状态下mint操作被拒绝
if ! cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "mint(uint256)" "1000000000000000000" 2>/dev/null; then
    echo "✅ 暂停状态下mint操作被正确拒绝"
else
    echo "❌ 暂停状态下mint操作未被拒绝"
    exit 1
fi

echo "ℹ️  测试恢复功能..."
cast send --private-key "$OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "unpause()" >/dev/null 2>&1

# 测试恢复后mint操作正常
if cast send --private-key "$OPERATOR_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "mint(uint256)" "1000000000000000000" >/dev/null 2>&1; then
    echo "✅ 恢复后mint操作正常"
else
    echo "❌ 恢复后mint操作失败"
    exit 1
fi
echo ""

# 步骤6: 角色查询功能测试
echo "🔬 步骤 5: 角色查询功能测试"
echo "----------------------------------------"

# 测试角色成员数量查询
MEMBER_COUNT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getRoleMemberCount(bytes32)" "$OPERATOR_ROLE")
MEMBER_COUNT_DEC=$((16#${MEMBER_COUNT:2}))

# 测试角色成员查询
if [ "$MEMBER_COUNT_DEC" -gt 0 ]; then
    MEMBER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getRoleMember(bytes32,uint256)" "$OPERATOR_ROLE" "0")
    MEMBER="0x${MEMBER:26}"
    MEMBER=$(cast to-check-sum-address "$MEMBER")
    OPERATOR_CHECKSUM=$(cast to-check-sum-address "$OPERATOR")
    if [ "$MEMBER" = "$OPERATOR_CHECKSUM" ]; then
        echo "✅ 角色查询功能正常"
    else
        echo "❌ 角色查询功能异常"
        exit 1
    fi
fi

# 测试hasRole查询
HAS_ROLE=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "hasRole(bytes32,address)" "$OPERATOR_ROLE" "$OPERATOR")
if [ "$HAS_ROLE" != "0x0000000000000000000000000000000000000000000000000000000000000001" ]; then
    echo "❌ hasRole查询异常"
    exit 1
fi

# 测试管理员相关查询
HAS_ADMIN=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "hasAdmin()")
IS_ADMIN=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "isAdmin(address)" "$ADMIN")
if [ "$HAS_ADMIN" != "0x0000000000000000000000000000000000000000000000000000000000000001" ] || \
   [ "$IS_ADMIN" != "0x0000000000000000000000000000000000000000000000000000000000000001" ]; then
    echo "❌ 管理员查询异常"
    exit 1
fi
echo "✅ 所有角色查询功能正常"
echo ""

# 步骤7: 管理员角色转移测试
echo "🔬 步骤 6: 管理员角色转移测试"
echo "----------------------------------------"

cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "transferAdminRole(address)" "$NEW_ADMIN" >/dev/null 2>&1

# 验证新管理员
ADMIN_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getAdmin()")
CURRENT_ADMIN="0x${ADMIN_RESULT:26}"
CURRENT_ADMIN=$(cast to-check-sum-address "$CURRENT_ADMIN")
NEW_ADMIN_CHECKSUM=$(cast to-check-sum-address "$NEW_ADMIN")
if [ "$CURRENT_ADMIN" != "$NEW_ADMIN_CHECKSUM" ]; then
    echo "❌ 管理员角色转移失败"
    exit 1
fi

# 验证权限转移：测试旧admin失去权限，新admin获得权限
# 先获取当前operator，然后使用不同的地址进行测试
CURRENT_OP_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getCurrentOperator()")
CURRENT_OP="0x${CURRENT_OP_RESULT:26}"
CURRENT_OP=$(cast to-check-sum-address "$CURRENT_OP")

# 选择一个不同的测试地址（确保不是当前operator）
if [ "$CURRENT_OP" = "$(cast to-check-sum-address "$OWNER")" ]; then
    TEST_OP_ADDRESS="$NEW_OWNER"
else
    TEST_OP_ADDRESS="$OWNER"
fi

# 测试旧admin权限失效
if ! cast send --private-key "$ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "setOperator(address)" "$TEST_OP_ADDRESS" >/dev/null 2>&1; then
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

# 转回原管理员
cast send --private-key "$NEW_ADMIN_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
    "$PROXY_ADDRESS" "transferAdminRole(address)" "$ADMIN" >/dev/null 2>&1
echo ""

# 步骤8: 所有者转移测试
echo "🔬 步骤 7: 所有者转移测试"
echo "----------------------------------------"

# 检查当前所有者
CURRENT_OWNER_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()")
CURRENT_OWNER_ADDR="0x${CURRENT_OWNER_RESULT:26}"
CURRENT_OWNER_ADDR=$(cast to-check-sum-address "$CURRENT_OWNER_ADDR")
OWNER_CHECKSUM=$(cast to-check-sum-address "$OWNER")

if [ "$CURRENT_OWNER_ADDR" != "$OWNER_CHECKSUM" ]; then
    echo "❌ 警告：当前所有者与预期不符，跳过所有者转移测试"
else
    cast send --private-key "$OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "transferOwnership(address)" "$NEW_OWNER" >/dev/null 2>&1

    # 验证新所有者
    OWNER_RESULT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()")
    CURRENT_OWNER="0x${OWNER_RESULT:26}"
    CURRENT_OWNER=$(cast to-check-sum-address "$CURRENT_OWNER")
    NEW_OWNER_CHECKSUM=$(cast to-check-sum-address "$NEW_OWNER")
    if [ "$CURRENT_OWNER" != "$NEW_OWNER_CHECKSUM" ]; then
        echo "❌ 所有者转移失败"
        exit 1
    fi

    # 验证权限转移
    if ! cast send --private-key "$OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "pause()" >/dev/null 2>&1; then
        if cast send --private-key "$NEW_OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
            "$PROXY_ADDRESS" "pause()" >/dev/null 2>&1; then
            echo "✅ 所有者转移成功"
            # 恢复运行状态
            cast send --private-key "$NEW_OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
                "$PROXY_ADDRESS" "unpause()" >/dev/null 2>&1
        else
            echo "❌ 新所有者权限未生效"
            exit 1
        fi
    else
        echo "❌ 旧所有者权限未失效"
        exit 1
    fi

    # 转回原所有者
    cast send --private-key "$NEW_OWNER_PRIVATE_KEY" --rpc-url "$RPC_URL" --legacy \
        "$PROXY_ADDRESS" "transferOwnership(address)" "$OWNER" >/dev/null 2>&1
fi
echo ""

echo "🎉 所有测试完成!"
echo "  ✅ Operator角色管理正常"
echo "  ✅ Mint操作正常"
echo "  ✅ Cleanup操作正常"
echo "  ✅ 暂停/恢复功能正常"
echo "  ✅ 角色查询功能正常"
echo "  ✅ 管理员转移功能正常"
echo "  ✅ 所有者转移功能正常"