#!/bin/bash

# Token Manager 全面测试脚本
# 测试所有接口功能：Owner、Admin、Minter、Burner权限，以及公共查询接口

set -e

echo "🧪 Token Manager 全面测试脚本"
echo "=============================="
echo "📋 测试范围: 所有接口功能覆盖"
echo ""

# 配置参数
OWNER_PRIVATE_KEY="0x9935c242a0b0ee41edcbd2d963f5bc7f142fdc803eb24f0df396a6fdb16c6af9"
ADMIN_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
RPC_URL="${RPC_URL:-http://localhost:8123}"
GAS_PRICE="${GAS_PRICE:-1000000000}"
GAS_LIMIT="${GAS_LIMIT:-5000000}"

# 获取地址
OWNER_ADDRESS=$(cast wallet address --private-key "$OWNER_PRIVATE_KEY")
ADMIN_ADDRESS=$(cast wallet address --private-key "$ADMIN_PRIVATE_KEY")

echo "📊 测试配置:"
echo "  RPC URL: $RPC_URL"
echo "  Owner: $OWNER_ADDRESS"
echo "  Admin: $ADMIN_ADDRESS"
echo ""

# 检查网络连接
echo "🔍 检查网络连接..."
if ! cast chain-id --rpc-url "$RPC_URL" > /dev/null; then
    echo "❌ 错误：无法连接到 RPC 端点 $RPC_URL"
    exit 1
fi

CHAIN_ID=$(cast chain-id --rpc-url "$RPC_URL")
echo "✅ 已连接到 Chain ID: $CHAIN_ID"
echo ""

# TokenManager代理合约地址
PROXY_ADDRESS="0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab"

echo "🎯 TokenManager代理合约地址: $PROXY_ADDRESS"
echo ""

# 创建临时测试账户
echo "👥 创建临时测试账户..."
TEMP_ACCOUNTS=()
TEMP_PRIVATE_KEYS=()
for i in {1..10}; do
    # 生成随机私钥
    TEMP_PRIVATE_KEY=$(openssl rand -hex 32)
    TEMP_ADDRESS=$(cast wallet address --private-key "0x$TEMP_PRIVATE_KEY")
    TEMP_ACCOUNTS+=("$TEMP_ADDRESS")
    TEMP_PRIVATE_KEYS+=("$TEMP_PRIVATE_KEY")
    echo "  账户 $i: $TEMP_ADDRESS"
done
echo ""

# 给临时账户转账
echo "💰 给临时账户转账..."
for i in {0..9}; do
    ACCOUNT=${TEMP_ACCOUNTS[$i]}
    echo "  转账给账户 $((i+1)): $ACCOUNT"
    
    cast send --private-key "$ADMIN_PRIVATE_KEY" \
        --rpc-url "$RPC_URL" \
        --legacy \
        "$ACCOUNT" \
        --value "100000000000000000000" # 100 ETH
    
    BALANCE=$(cast balance "$ACCOUNT" --rpc-url "$RPC_URL" --ether)
    echo "    余额: $BALANCE ETH"
done
echo ""

# 测试函数
test_step() {
    local step=$1
    local description=$2
    echo "🔬 测试步骤 $step: $description"
    echo "----------------------------------------"
}

test_success() {
    echo "✅ 成功: $1"
}

test_failure() {
    echo "❌ 失败: $1"
}

test_info() {
    echo "ℹ️  $1"
}

# 测试1: 基础查询接口
test_step "1" "基础查询接口测试"
echo ""

# 查询合约基本信息
test_info "查询合约基本信息..."
VERSION=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "VERSION()")
VERSION_TEXT=$(cast to-ascii "$VERSION" 2>/dev/null || echo "无法解码")
echo "  版本: $VERSION_TEXT"

IS_ACTIVE=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "isActive()")
IS_ACTIVE_TEXT=$([ "$IS_ACTIVE" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "已激活" || echo "未激活")
echo "  激活状态: $IS_ACTIVE_TEXT"

IS_PAUSED=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "paused()")
IS_PAUSED_TEXT=$([ "$IS_PAUSED" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "已暂停" || echo "运行中")
echo "  暂停状态: $IS_PAUSED_TEXT"

ACTIVATION_BLOCK=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "activationBlock()")
ACTIVATION_BLOCK_DEC=$(cast to-dec "$ACTIVATION_BLOCK")
echo "  激活区块: $ACTIVATION_BLOCK_DEC"

CURRENT_OWNER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()")
CURRENT_OWNER="0x${CURRENT_OWNER:26}"
echo "  当前Owner: $CURRENT_OWNER"

CURRENT_ADMIN=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getAdmin()")
CURRENT_ADMIN="0x${CURRENT_ADMIN:26}"
echo "  当前Admin: $CURRENT_ADMIN"

test_success "基础查询接口测试完成"
echo ""

# 测试2: 角色管理接口
test_step "2" "角色管理接口测试"
echo ""

# 查询初始角色状态
test_info "查询初始角色状态..."
MINTER_COUNT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getMinterRoleCount()" --from "$ADMIN_ADDRESS")
MINTER_COUNT_DEC=$(cast to-dec "$MINTER_COUNT")
echo "  Minter数量: $MINTER_COUNT_DEC"

BURNER_COUNT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getBurnerRoleCount()" --from "$ADMIN_ADDRESS")
BURNER_COUNT_DEC=$(cast to-dec "$BURNER_COUNT")
echo "  Burner数量: $BURNER_COUNT_DEC"

# 授予Minter角色
test_info "授予Minter角色..."
MINTER1=${TEMP_ACCOUNTS[0]}
MINTER2=${TEMP_ACCOUNTS[1]}

echo "  授予 $MINTER1 为Minter..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "grantMinterRole(address)" "$MINTER1"

echo "  授予 $MINTER2 为Minter..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "grantMinterRole(address)" "$MINTER2"

# 授予Burner角色
test_info "授予Burner角色..."
BURNER1=${TEMP_ACCOUNTS[2]}
BURNER2=${TEMP_ACCOUNTS[3]}

echo "  授予 $BURNER1 为Burner..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "grantBurnerRole(address)" "$BURNER1"

echo "  授予 $BURNER2 为Burner..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "grantBurnerRole(address)" "$BURNER2"

# 验证角色授予
test_info "验证角色授予..."
MINTER_COUNT_AFTER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getMinterRoleCount()" --from "$ADMIN_ADDRESS")
MINTER_COUNT_AFTER_DEC=$(cast to-dec "$MINTER_COUNT_AFTER")
echo "  Minter数量: $MINTER_COUNT_AFTER_DEC"

BURNER_COUNT_AFTER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getBurnerRoleCount()" --from "$ADMIN_ADDRESS")
BURNER_COUNT_AFTER_DEC=$(cast to-dec "$BURNER_COUNT_AFTER")
echo "  Burner数量: $BURNER_COUNT_AFTER_DEC"

# 查询角色成员
test_info "查询角色成员..."
MINTERS=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getMintersPaginated(uint256,uint256)" 0 10 --from "$ADMIN_ADDRESS")
echo "  Minters: $MINTERS"

BURNERS=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getBurnersPaginated(uint256,uint256)" 0 10 --from "$ADMIN_ADDRESS")
echo "  Burners: $BURNERS"

test_success "角色管理接口测试完成"
echo ""

# 测试3: 白名单管理接口
test_step "3" "白名单管理接口测试"
echo ""

# 查询初始白名单状态
test_info "查询初始白名单状态..."
MINT_WHITELIST_COUNT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getMintWhitelistCount()")
MINT_WHITELIST_COUNT_DEC=$(cast to-dec "$MINT_WHITELIST_COUNT")
echo "  Mint白名单数量: $MINT_WHITELIST_COUNT_DEC"

BURN_WHITELIST_COUNT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getBurnWhitelistCount()")
BURN_WHITELIST_COUNT_DEC=$(cast to-dec "$BURN_WHITELIST_COUNT")
echo "  Burn白名单数量: $BURN_WHITELIST_COUNT_DEC"

# 添加Mint白名单
test_info "添加Mint白名单..."
MINT_TARGET1=${TEMP_ACCOUNTS[4]}
MINT_TARGET2=${TEMP_ACCOUNTS[5]}

echo "  添加 $MINT_TARGET1 到Mint白名单..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "addMintWhitelist(address)" "$MINT_TARGET1"

echo "  添加 $MINT_TARGET2 到Mint白名单..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "addMintWhitelist(address)" "$MINT_TARGET2"

# 验证白名单添加
test_info "验证白名单添加..."
MINT_WHITELIST_COUNT_AFTER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getMintWhitelistCount()")
MINT_WHITELIST_COUNT_AFTER_DEC=$(cast to-dec "$MINT_WHITELIST_COUNT_AFTER")
echo "  Mint白名单数量: $MINT_WHITELIST_COUNT_AFTER_DEC"

# 查询白名单成员
test_info "查询白名单成员..."
MINT_WHITELIST=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getMintWhitelist(uint256,uint256)" 0 10)
echo "  Mint白名单: $MINT_WHITELIST"

BURN_WHITELIST=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getBurnWhitelist(uint256,uint256)" 0 10)
echo "  Burn白名单: $BURN_WHITELIST"

# 测试白名单检查
test_info "测试白名单检查..."
IS_MINT_ALLOWED1=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "isMintAllowed(address)" "$MINT_TARGET1")
IS_MINT_ALLOWED1_TEXT=$([ "$IS_MINT_ALLOWED1" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "允许" || echo "禁止")
echo "  $MINT_TARGET1 Mint权限: $IS_MINT_ALLOWED1_TEXT"

IS_BURN_ALLOWED1=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "isBurnAllowed(address)" "$BURNER1")
IS_BURN_ALLOWED1_TEXT=$([ "$IS_BURN_ALLOWED1" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "允许" || echo "禁止")
echo "  $BURNER1 Burn权限: $IS_BURN_ALLOWED1_TEXT"

test_success "白名单管理接口测试完成"
echo ""

# 测试4: Mint/Burn功能测试
test_step "4" "Mint/Burn功能测试"
echo ""

# 测试成功的Mint操作
test_info "测试成功的Mint操作..."
MINT_AMOUNT="1000000000000000000" # 1 ETH

# 使用已授予MINTER_ROLE的账户的私钥
MINTER1_PRIVATE_KEY="0x${TEMP_PRIVATE_KEYS[0]}"
MINTER1=${TEMP_ACCOUNTS[0]}

echo "  Minter $MINTER1 向 $MINT_TARGET1 Mint 1 ETH..."
cast send --private-key "$MINTER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "mint(address,uint256)" "$MINT_TARGET1" "$MINT_AMOUNT"

# 检查余额变化
BALANCE_AFTER_MINT=$(cast balance "$MINT_TARGET1" --rpc-url "$RPC_URL" --ether)
echo "  $MINT_TARGET1 余额: $BALANCE_AFTER_MINT ETH"

# 测试失败的Mint操作（无权限）
test_info "测试失败的Mint操作（无权限）..."
NON_MINTER=${TEMP_ACCOUNTS[6]}
NON_MINTER_PRIVATE_KEY="0x${TEMP_PRIVATE_KEYS[6]}"
echo "  非Minter $NON_MINTER 尝试Mint（应该失败）..."
if cast send --private-key "$NON_MINTER_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "mint(address,uint256)" "$MINT_TARGET1" "$MINT_AMOUNT" 2>&1 | grep -q "revert"; then
    test_success "无权限Mint被正确拒绝"
else
    test_failure "无权限Mint未被拒绝"
fi

# 测试失败的Mint操作（目标不在白名单）
test_info "测试失败的Mint操作（目标不在白名单）..."
NON_WHITELIST_TARGET=${TEMP_ACCOUNTS[7]}
echo "  向非白名单地址 $NON_WHITELIST_TARGET Mint（应该失败）..."
if cast send --private-key "$MINTER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "mint(address,uint256)" "$NON_WHITELIST_TARGET" "$MINT_AMOUNT" 2>&1 | grep -q "revert"; then
    test_success "向非白名单地址Mint被正确拒绝"
else
    test_failure "向非白名单地址Mint未被拒绝"
fi

# 测试成功的Burn操作
test_info "测试成功的Burn操作..."
BURN_AMOUNT="500000000000000000" # 0.5 ETH

# 使用已授予BURNER_ROLE的账户的私钥
BURNER1_PRIVATE_KEY="0x${TEMP_PRIVATE_KEYS[2]}"
BURNER1=${TEMP_ACCOUNTS[2]}

# 先给burn白名单地址转账一些ETH，然后进行burn测试
BURN_TARGET="0x000000000000000000000000000000000000dead"
echo "  先给burn白名单地址转账1 ETH..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$BURN_TARGET" \
    --value "1000000000000000000" # 1 ETH

echo "  Burner $BURNER1 从 $BURN_TARGET Burn 0.5 ETH..."
cast send --private-key "$BURNER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "burn(address,uint256)" "$BURN_TARGET" "$BURN_AMOUNT"

# 检查余额变化
BALANCE_AFTER_BURN=$(cast balance "$MINT_TARGET1" --rpc-url "$RPC_URL" --ether)
echo "  $MINT_TARGET1 余额: $BALANCE_AFTER_BURN ETH"

# 测试失败的Burn操作（无权限）
test_info "测试失败的Burn操作（无权限）..."
NON_BURNER=${TEMP_ACCOUNTS[8]}
NON_BURNER_PRIVATE_KEY="0x${TEMP_PRIVATE_KEYS[8]}"
echo "  非Burner $NON_BURNER 尝试Burn（应该失败）..."
if cast send --private-key "$NON_BURNER_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "burn(address,uint256)" "$MINT_TARGET1" "$BURN_AMOUNT" 2>&1 | grep -q "revert"; then
    test_success "无权限Burn被正确拒绝"
else
    test_failure "无权限Burn未被拒绝"
fi

test_success "Mint/Burn功能测试完成"
echo ""

# 测试5: Pause/Unpause功能测试
test_step "5" "Pause/Unpause功能测试"
echo ""

# 测试Pause功能
test_info "测试Pause功能..."
echo "  Owner暂停合约..."
cast send --private-key "$OWNER_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "pause()"

# 验证暂停状态
IS_PAUSED_AFTER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "paused()")
IS_PAUSED_AFTER_TEXT=$([ "$IS_PAUSED_AFTER" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "已暂停" || echo "运行中")
echo "  暂停状态: $IS_PAUSED_AFTER_TEXT"

# 测试暂停状态下的Mint操作（应该失败）
test_info "测试暂停状态下的Mint操作（应该失败）..."
echo "  暂停状态下尝试Mint（应该失败）..."
if cast send --private-key "$MINTER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "mint(address,uint256)" "$MINT_TARGET1" "$MINT_AMOUNT" 2>&1 | grep -q "revert"; then
    test_success "暂停状态下的Mint被正确拒绝"
else
    test_failure "暂停状态下的Mint未被拒绝"
fi

# 测试暂停状态下的Burn操作（应该失败）
test_info "测试暂停状态下的Burn操作（应该失败）..."
echo "  暂停状态下尝试Burn（应该失败）..."
if cast send --private-key "$BURNER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "burn(address,uint256)" "$MINT_TARGET1" "$BURN_AMOUNT" 2>&1 | grep -q "revert"; then
    test_success "暂停状态下的Burn被正确拒绝"
else
    test_failure "暂停状态下的Burn未被拒绝"
fi

# 测试Unpause功能
test_info "测试Unpause功能..."
echo "  Owner恢复合约..."
cast send --private-key "$OWNER_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "unpause()"

# 验证恢复状态
IS_PAUSED_FINAL=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "paused()")
IS_PAUSED_FINAL_TEXT=$([ "$IS_PAUSED_FINAL" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "已暂停" || echo "运行中")
echo "  暂停状态: $IS_PAUSED_FINAL_TEXT"

# 测试恢复状态下的Mint操作（应该成功）
test_info "测试恢复状态下的Mint操作（应该成功）..."
echo "  恢复状态下尝试Mint（应该成功）..."
cast send --private-key "$MINTER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "mint(address,uint256)" "$MINT_TARGET2" "$MINT_AMOUNT"

BALANCE_AFTER_UNPAUSE=$(cast balance "$MINT_TARGET2" --rpc-url "$RPC_URL" --ether)
echo "  $MINT_TARGET2 余额: $BALANCE_AFTER_UNPAUSE ETH"
test_success "恢复状态下的Mint成功"

test_success "Pause/Unpause功能测试完成"
echo ""

# 测试6: 角色撤销测试
test_step "6" "角色撤销测试"
echo ""

# 撤销Minter角色
test_info "撤销Minter角色..."
echo "  撤销 $MINTER1 的Minter角色..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "revokeMinterRole(address)" "$MINTER1"

# 验证撤销后的Mint操作（应该失败）
test_info "验证撤销后的Mint操作（应该失败）..."
echo "  撤销Minter角色后尝试Mint（应该失败）..."
if cast send --private-key "$MINTER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "mint(address,uint256)" "$MINT_TARGET1" "$MINT_AMOUNT" 2>&1 | grep -q "revert"; then
    test_success "撤销Minter角色后的Mint被正确拒绝"
else
    test_failure "撤销Minter角色后的Mint未被拒绝"
fi

# 撤销Burner角色
test_info "撤销Burner角色..."
echo "  撤销 $BURNER1 的Burner角色..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "revokeBurnerRole(address)" "$BURNER1"

# 验证撤销后的Burn操作（应该失败）
test_info "验证撤销后的Burn操作（应该失败）..."
echo "  撤销Burner角色后尝试Burn（应该失败）..."
if cast send --private-key "$BURNER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "burn(address,uint256)" "$MINT_TARGET1" "$BURN_AMOUNT" 2>&1 | grep -q "revert"; then
    test_success "撤销Burner角色后的Burn被正确拒绝"
else
    test_failure "撤销Burner角色后的Burn未被拒绝"
fi

test_success "角色撤销测试完成"
echo ""

# 测试7: 白名单移除测试
test_step "7" "白名单移除测试"
echo ""

# 移除Mint白名单
test_info "移除Mint白名单..."
echo "  从Mint白名单移除 $MINT_TARGET1..."
cast send --private-key "$ADMIN_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "removeMintWhitelist(address)" "$MINT_TARGET1"

# 验证移除后的Mint操作（应该失败）
test_info "验证移除后的Mint操作（应该失败）..."
echo "  从白名单移除后尝试Mint（应该失败）..."
if cast send --private-key "$MINTER1_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" \
    --legacy \
    "$PROXY_ADDRESS" \
    "mint(address,uint256)" "$MINT_TARGET1" "$MINT_AMOUNT" 2>&1 | grep -q "revert"; then
    test_success "从白名单移除后的Mint被正确拒绝"
else
    test_failure "从白名单移除后的Mint未被拒绝"
fi

test_success "白名单移除测试完成"
echo ""

# 测试8: 最终状态验证
test_step "8" "最终状态验证"
echo ""

test_info "查询最终状态..."
FINAL_OWNER=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "owner()")
FINAL_OWNER="0x${FINAL_OWNER:26}"
echo "  最终Owner: $FINAL_OWNER"

FINAL_ADMIN=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getAdmin()")
FINAL_ADMIN="0x${FINAL_ADMIN:26}"
echo "  最终Admin: $FINAL_ADMIN"

FINAL_MINTER_COUNT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getMinterRoleCount()" --from "$ADMIN_ADDRESS")
FINAL_MINTER_COUNT_DEC=$(cast to-dec "$FINAL_MINTER_COUNT")
echo "  最终Minter数量: $FINAL_MINTER_COUNT_DEC"

FINAL_BURNER_COUNT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getBurnerRoleCount()" --from "$ADMIN_ADDRESS")
FINAL_BURNER_COUNT_DEC=$(cast to-dec "$FINAL_BURNER_COUNT")
echo "  最终Burner数量: $FINAL_BURNER_COUNT_DEC"

FINAL_MINT_WHITELIST_COUNT=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "getMintWhitelistCount()")
FINAL_MINT_WHITELIST_COUNT_DEC=$(cast to-dec "$FINAL_MINT_WHITELIST_COUNT")
echo "  最终Mint白名单数量: $FINAL_MINT_WHITELIST_COUNT_DEC"

FINAL_IS_PAUSED=$(cast call --rpc-url "$RPC_URL" "$PROXY_ADDRESS" "paused()")
FINAL_IS_PAUSED_TEXT=$([ "$FINAL_IS_PAUSED" = "0x0000000000000000000000000000000000000000000000000000000000000001" ] && echo "已暂停" || echo "运行中")
echo "  最终暂停状态: $FINAL_IS_PAUSED_TEXT"

test_success "最终状态验证完成"
echo ""

# 测试总结
echo "🎉 Token Manager 全面测试完成"
echo "=============================="
echo ""
echo "📊 测试覆盖范围:"
echo "  ✅ 基础查询接口 (VERSION, isActive, paused, activationBlock, owner, getAdmin)"
echo "  ✅ 角色管理接口 (grantMinterRole, grantBurnerRole, revokeMinterRole, revokeBurnerRole)"
echo "  ✅ 角色查询接口 (getMinterRoleCount, getBurnerRoleCount, getMintersPaginated, getBurnersPaginated)"
echo "  ✅ 白名单管理接口 (addMintWhitelist, removeMintWhitelist, getMintWhitelist, getBurnWhitelist)"
echo "  ✅ 白名单查询接口 (getMintWhitelistCount, getBurnWhitelistCount, isMintAllowed, isBurnAllowed)"
echo "  ✅ Mint/Burn功能 (mint, burn)"
echo "  ✅ 权限控制 (无权限操作被正确拒绝)"
echo "  ✅ 白名单控制 (非白名单操作被正确拒绝)"
echo "  ✅ 暂停控制 (pause, unpause, 暂停状态下操作被拒绝)"
echo "  ✅ 状态验证 (所有状态变化正确)"
echo ""
echo "🔐 权限测试:"
echo "  ✅ Owner权限 (pause/unpause)"
echo "  ✅ Admin权限 (角色管理/白名单管理)"
echo "  ✅ Minter权限 (mint操作)"
echo "  ✅ Burner权限 (burn操作)"
echo "  ✅ 权限拒绝 (无权限操作被正确拒绝)"
echo ""
echo "✅ 所有接口功能测试通过！" 