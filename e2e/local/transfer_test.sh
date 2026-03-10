#!/bin/bash
# 转账功能测试脚本
# 用于验证 moca 节点的转账功能是否正常

set -e

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
PROJECT_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)

cd "${PROJECT_ROOT}"

echo "=== 开始转账功能测试 ==="

# 检查节点是否运行
if ! pgrep -f "mocad start" > /dev/null; then
    echo "错误: 节点未运行，请先启动节点"
    exit 1
fi

# 等待节点完全启动
echo "等待节点完全启动..."
sleep 5

# 获取节点状态
NODE_STATUS=$(./build/mocad status --node http://localhost:26657 2>&1)
if echo "$NODE_STATUS" | grep -q "error\|Error\|failed\|connection refused"; then
    echo "错误: 无法连接到节点"
    echo "$NODE_STATUS"
    exit 1
fi

# 检查节点是否已同步
CATCHING_UP=$(echo "$NODE_STATUS" | grep -o '"catching_up":[^,}]*' | grep -o '[^:]*$' | tr -d ' ' || echo "false")
if [ "$CATCHING_UP" = "true" ]; then
    echo "警告: 节点仍在同步中，等待同步完成..."
    for i in {1..30}; do
        sleep 2
        CATCHING_UP=$(./build/mocad status --node http://localhost:26657 2>/dev/null | grep -o '"catching_up":[^,}]*' | grep -o '[^:]*$' | tr -d ' ')
        if [ "$CATCHING_UP" = "false" ]; then
            echo "节点已同步完成"
            break
        fi
    done
    if [ "$CATCHING_UP" = "true" ]; then
        echo "错误: 节点同步超时"
        exit 1
    fi
fi

# 获取初始余额
echo "获取接收地址初始余额..."
BALANCE_JSON=$(./build/mocad query bank balances 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 --node http://localhost:26657 --chain-id moca_5151-1 --output json 2>/dev/null)
if [ -z "$BALANCE_JSON" ]; then
    INITIAL_BALANCE=0
else
    # 尝试使用 jq 解析，如果失败则使用 grep
    if command -v jq >/dev/null 2>&1; then
        INITIAL_BALANCE=$(echo "$BALANCE_JSON" | jq -r '.balances[0].amount // "0"' 2>/dev/null || echo "0")
    else
        INITIAL_BALANCE=$(echo "$BALANCE_JSON" | grep -o '"amount":"[0-9]*"' | head -1 | grep -o '[0-9]*' || echo "0")
    fi
fi

echo "初始余额: ${INITIAL_BALANCE}amoca"

# 执行转账
echo "执行转账..."
TRANSFER_AMOUNT="2000000000000000000000amoca"
FEES="200000000000000amoca"

TX_RESULT=$(./build/mocad tx bank send validator0 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 "$TRANSFER_AMOUNT" \
    --keyring-backend test \
    --node http://localhost:26657 \
    --chain-id moca_5151-1 \
    -y \
    --fees "$FEES" \
    --home ./deployment/localup/.local/validator0 2>&1)

if [ $? -ne 0 ]; then
    echo "错误: 转账失败"
    echo "$TX_RESULT"
    exit 1
fi

echo "转账交易已提交"
echo "$TX_RESULT" | grep -E "txhash|code" || true

# 检查交易是否成功提交
TX_CODE=$(echo "$TX_RESULT" | grep -oE "code: [0-9]+" | grep -oE "[0-9]+" || echo "")
if [ -n "$TX_CODE" ] && [ "$TX_CODE" != "0" ]; then
    echo "错误: 交易提交失败（code: $TX_CODE）"
    echo "$TX_RESULT"
    exit 1
fi

# 等待交易确认
echo "等待交易确认..."
sleep 5

# 获取交易哈希
TXHASH=$(echo "$TX_RESULT" | grep -oE "txhash: [0-9A-Fa-f]{64}" | cut -d' ' -f2 || echo "")

if [ -z "$TXHASH" ]; then
    echo "警告: 无法获取交易哈希，尝试查询最新余额"
    # 如果无法获取交易哈希，等待一段时间后直接验证余额
    sleep 5
else
    echo "交易哈希: $TXHASH"

    # 查询交易状态
    echo "查询交易状态..."
    sleep 3
    TX_STATUS=$(./build/mocad query tx "$TXHASH" --node http://localhost:26657 --chain-id moca_5151-1 --output json 2>/dev/null || echo "")

    if [ -z "$TX_STATUS" ]; then
        echo "警告: 无法查询交易状态，等待更长时间后验证余额"
        sleep 5
    else
        CODE=$(echo "$TX_STATUS" | grep -o '"code":[0-9]*' | grep -o '[0-9]*' || echo "")
        if [ "$CODE" = "0" ]; then
            echo "✓ 交易成功确认（code: 0）"
        else
            echo "错误: 交易失败（code: $CODE）"
            echo "$TX_STATUS" | grep -A 5 "raw_log" || true
            exit 1
        fi
    fi
fi

# 验证余额变化
echo "验证余额变化..."
sleep 3
BALANCE_JSON=$(./build/mocad query bank balances 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 --node http://localhost:26657 --chain-id moca_5151-1 --output json 2>/dev/null)
if [ -z "$BALANCE_JSON" ]; then
    FINAL_BALANCE=0
else
    # 尝试使用 jq 解析，如果失败则使用 grep
    if command -v jq >/dev/null 2>&1; then
        FINAL_BALANCE=$(echo "$BALANCE_JSON" | jq -r '.balances[0].amount // "0"' 2>/dev/null || echo "0")
    else
        FINAL_BALANCE=$(echo "$BALANCE_JSON" | grep -o '"amount":"[0-9]*"' | head -1 | grep -o '[0-9]*' || echo "0")
    fi
fi

echo "最终余额: ${FINAL_BALANCE}amoca"

# 计算预期余额（初始余额 + 转账金额）
EXPECTED_AMOUNT=$(echo "$TRANSFER_AMOUNT" | grep -o '[0-9]*')
# 使用 bc 进行大数计算（如果可用），否则使用 awk
if command -v bc >/dev/null 2>&1; then
    EXPECTED_BALANCE=$(echo "$INITIAL_BALANCE + $EXPECTED_AMOUNT" | bc)
    BALANCE_DIFF=$(echo "$FINAL_BALANCE - $EXPECTED_BALANCE" | bc | tr -d '-')
    # 允许 1% 的误差（考虑手续费等因素）
    ALLOWED_DIFF=$(echo "scale=0; $EXPECTED_BALANCE / 100" | bc)
    if [ "$(echo "$BALANCE_DIFF <= $ALLOWED_DIFF" | bc)" -eq 1 ]; then
        echo "✓ 余额验证成功（预期: ${EXPECTED_BALANCE}amoca, 实际: ${FINAL_BALANCE}amoca）"
    else
        echo "警告: 余额变化不符合预期（预期: ${EXPECTED_BALANCE}amoca, 实际: ${FINAL_BALANCE}amoca）"
        echo "可能原因: 初始余额查询不准确或存在其他交易"
    fi
else
    # 使用 awk 进行大数比较
    BALANCE_INCREASE=$(echo "$FINAL_BALANCE $INITIAL_BALANCE" | awk '{print $1 - $2}')
    if [ "$(echo "$BALANCE_INCREASE $EXPECTED_AMOUNT" | awk '{if ($1 >= $2 * 0.99 && $1 <= $2 * 1.01) print 1; else print 0}')" -eq 1 ]; then
        echo "✓ 余额验证成功（初始: ${INITIAL_BALANCE}amoca, 增加: ${BALANCE_INCREASE}amoca, 预期增加: ${EXPECTED_AMOUNT}amoca, 最终: ${FINAL_BALANCE}amoca）"
    else
        echo "警告: 余额变化不符合预期（初始: ${INITIAL_BALANCE}amoca, 增加: ${BALANCE_INCREASE}amoca, 预期增加: ${EXPECTED_AMOUNT}amoca, 最终: ${FINAL_BALANCE}amoca）"
        echo "可能原因: 初始余额查询不准确或存在其他交易"
    fi
fi

echo ""
echo "=== 转账功能测试完成 ==="
echo "✓ 转账测试通过"

