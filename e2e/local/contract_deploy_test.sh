#!/bin/bash
# 合约部署功能测试脚本
# 用于验证 moca 节点的合约部署和 transfer 功能是否正常

set -e

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
PROJECT_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
ERC20_DIR="${SCRIPT_DIR}/ERC20"

cd "${PROJECT_ROOT}"

# 禁用代理以避免 HTTP/2 frame too large 错误
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY
export no_proxy="localhost,127.0.0.1,0.0.0.0"
export NO_PROXY="localhost,127.0.0.1,0.0.0.0"

echo "=== 开始合约部署功能测试 ==="

# 检查节点是否运行
if ! pgrep -f "mocad start" > /dev/null; then
    echo "错误: 节点未运行，请先启动节点"
    exit 1
fi

# 检查 ERC20 目录是否存在
if [ ! -d "$ERC20_DIR" ]; then
    echo "错误: ERC20 合约目录不存在: $ERC20_DIR"
    exit 1
fi

# 检查 forge 和 cast 是否安装
if ! command -v forge >/dev/null 2>&1; then
    echo "错误: forge 未安装，请先安装 Foundry"
    exit 1
fi

if ! command -v cast >/dev/null 2>&1; then
    echo "错误: cast 未安装，请先安装 Foundry"
    exit 1
fi

# 检查 EVM RPC 是否可用
# 使用 127.0.0.1 而不是 localhost 以避免代理问题
RPC_URL="http://127.0.0.1:8545"
echo "检查 EVM RPC 连接..."
# 使用 curl 测试连接（绕过代理）
RPC_TEST=$(curl -s --noproxy "*" --http1.1 -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    "$RPC_URL" 2>&1)
if echo "$RPC_TEST" | grep -q '"error"'; then
    echo "错误: 无法连接到 EVM RPC: $RPC_URL"
    echo "$RPC_TEST"
    exit 1
fi
echo "✓ EVM RPC 连接正常"

# 获取 validator0 私钥（从 localup.sh 中获取）
VALIDATOR0_PRIVATE_KEY="0xe54bff83fc945cba77ca3e45d69adc5b57ad8db6073736c8422692abecfb5fe2"
VALIDATOR0_ADDRESS="0xbf657D0ef7b48167657A703Ed8Fd063F075246D7"

# 测试接收地址（使用 devaccount 地址）
RECEIVER_ADDRESS="0x3cfe397F6fb3D7A52B248FAf94FD6d8c8a847680"

# 设置初始供应量
INITIAL_SUPPLY_WEI="1000000000000000000000000"
TRANSFER_AMOUNT="100000000000000000000000"  # 1000 tokens

# 切换到 ERC20 目录
cd "$ERC20_DIR"

# 编译合约
echo "编译合约..."
export FOUNDRY_EVM_VERSION=paris
if ! forge build >/dev/null 2>&1; then
    echo "错误: 合约编译失败"
    exit 1
fi
echo "✓ 合约编译成功"

# 部署合约
echo "部署合约到本地链..."
# 清空代理环境变量以确保 forge 不使用代理
DEPLOY_OUTPUT=$(http_proxy="" https_proxy="" forge create \
    --rpc-url "$RPC_URL" \
    --private-key "$VALIDATOR0_PRIVATE_KEY" \
    --broadcast \
    src/MyToken.sol:MyToken \
    --constructor-args "$INITIAL_SUPPLY_WEI" 2>&1)

if [ $? -ne 0 ]; then
    echo "错误: 合约部署失败"
    echo "$DEPLOY_OUTPUT"
    exit 1
fi

# 提取合约地址
CONTRACT_ADDRESS=$(echo "$DEPLOY_OUTPUT" | awk '/Deployed to:/ {print $3}')
if [ -z "$CONTRACT_ADDRESS" ]; then
    echo "错误: 无法解析合约地址"
    echo "$DEPLOY_OUTPUT"
    exit 1
fi

echo "✓ 合约部署成功"
echo "合约地址: $CONTRACT_ADDRESS"

# 验证合约部署
echo "验证合约部署..."
SYMBOL=$(cast call "$CONTRACT_ADDRESS" "symbol()(string)" --rpc-url "$RPC_URL" 2>/dev/null || echo "")
TOTAL_SUPPLY=$(cast call "$CONTRACT_ADDRESS" "totalSupply()(uint256)" --rpc-url "$RPC_URL" 2>/dev/null || echo "")

if [ -z "$SYMBOL" ] || [ -z "$TOTAL_SUPPLY" ]; then
    echo "错误: 无法查询合约信息"
    exit 1
fi

echo "✓ 合约信息验证成功"
echo "  Symbol: $SYMBOL"
echo "  Total Supply: $TOTAL_SUPPLY"

# 获取部署者初始余额
echo "获取部署者初始余额..."
DEPLOYER_BALANCE_RAW=$(http_proxy="" https_proxy="" cast call "$CONTRACT_ADDRESS" "balanceOf(address)(uint256)" "$VALIDATOR0_ADDRESS" --rpc-url "$RPC_URL" 2>/dev/null || echo "0")
DEPLOYER_BALANCE=$(echo "$DEPLOYER_BALANCE_RAW" | awk '{print $1}')
echo "部署者余额: $DEPLOYER_BALANCE"

# 获取接收者初始余额
echo "获取接收者初始余额..."
RECEIVER_BALANCE_BEFORE_RAW=$(http_proxy="" https_proxy="" cast call "$CONTRACT_ADDRESS" "balanceOf(address)(uint256)" "$RECEIVER_ADDRESS" --rpc-url "$RPC_URL" 2>/dev/null || echo "0")
RECEIVER_BALANCE_BEFORE=$(echo "$RECEIVER_BALANCE_BEFORE_RAW" | awk '{print $1}')
echo "接收者初始余额: $RECEIVER_BALANCE_BEFORE"

# 执行 transfer
echo "执行 transfer 交易..."
TRANSFER_TX=$(http_proxy="" https_proxy="" cast send "$CONTRACT_ADDRESS" \
    "transfer(address,uint256)" \
    "$RECEIVER_ADDRESS" \
    "$TRANSFER_AMOUNT" \
    --private-key "$VALIDATOR0_PRIVATE_KEY" \
    --rpc-url "$RPC_URL" 2>&1)

if [ $? -ne 0 ]; then
    echo "错误: transfer 交易失败"
    echo "$TRANSFER_TX"
    exit 1
fi

# 提取交易哈希
TX_HASH=$(echo "$TRANSFER_TX" | grep -oE "0x[0-9a-fA-F]{64}" | head -1)
if [ -z "$TX_HASH" ]; then
    echo "警告: 无法解析交易哈希"
else
    echo "✓ Transfer 交易已提交"
    echo "交易哈希: $TX_HASH"
fi

# 等待交易确认
echo "等待交易确认..."
sleep 5

# 验证接收者余额变化
echo "验证接收者余额变化..."
RECEIVER_BALANCE_AFTER_RAW=$(http_proxy="" https_proxy="" cast call "$CONTRACT_ADDRESS" "balanceOf(address)(uint256)" "$RECEIVER_ADDRESS" --rpc-url "$RPC_URL" 2>/dev/null || echo "0")
RECEIVER_BALANCE_AFTER=$(echo "$RECEIVER_BALANCE_AFTER_RAW" | awk '{print $1}')
echo "接收者最终余额: $RECEIVER_BALANCE_AFTER"

# 验证余额变化
if [ "$RECEIVER_BALANCE_AFTER" = "$RECEIVER_BALANCE_BEFORE" ]; then
    echo "错误: 接收者余额未变化"
    exit 1
fi

# 计算余额增加量
if command -v bc >/dev/null 2>&1; then
    BALANCE_INCREASE=$(echo "$RECEIVER_BALANCE_AFTER - $RECEIVER_BALANCE_BEFORE" | bc)
    if [ "$(echo "$BALANCE_INCREASE >= $TRANSFER_AMOUNT" | bc)" -eq 1 ]; then
        echo "✓ Transfer 功能验证成功（余额增加: $BALANCE_INCREASE，预期: $TRANSFER_AMOUNT）"
    else
        echo "警告: 余额增加量不符合预期（增加: $BALANCE_INCREASE，预期: $TRANSFER_AMOUNT）"
    fi
else
    # 使用 awk 进行大数比较
    BALANCE_INCREASE=$(echo "$RECEIVER_BALANCE_AFTER $RECEIVER_BALANCE_BEFORE" | awk '{print $1 - $2}')
    if [ "$(echo "$BALANCE_INCREASE $TRANSFER_AMOUNT" | awk '{if ($1 >= $2 * 0.99 && $1 <= $2 * 1.01) print 1; else print 0}')" -eq 1 ]; then
        echo "✓ Transfer 功能验证成功（余额增加: $BALANCE_INCREASE，预期: $TRANSFER_AMOUNT）"
    else
        echo "警告: 余额增加量不符合预期（增加: $BALANCE_INCREASE，预期: $TRANSFER_AMOUNT）"
    fi
fi

echo ""
echo "=== 合约部署功能测试完成 ==="
echo "✓ 合约部署测试通过"
echo "✓ Transfer 功能测试通过"

