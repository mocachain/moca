#!/bin/bash

# 测试两个 EVM RPC 端点的块高度是否一致

SG_EVM_RPC="https://testnet-rpc-sg.mocachain.org"
EU_EVM_RPC="https://testnet-rpc-eu.mocachain.org"

echo "正在使用 EVM RPC 查询块高度..."
echo ""

# 使用 EVM RPC 的 eth_blockNumber 方法
echo "方法: 使用 EVM RPC (eth_blockNumber)"

# 查询 SG 端点的块高度
SG_RESPONSE=$(curl -s --connect-timeout 10 -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  ${SG_EVM_RPC} 2>/dev/null)

SG_HEIGHT_HEX=$(echo "$SG_RESPONSE" | jq -r '.result // empty')
SG_HEIGHT=""

if [ -n "$SG_HEIGHT_HEX" ] && [ "$SG_HEIGHT_HEX" != "null" ]; then
    # 将十六进制转换为十进制
    SG_HEIGHT=$(printf "%d" $SG_HEIGHT_HEX 2>/dev/null)
fi

# 查询 EU 端点的块高度
EU_RESPONSE=$(curl -s --connect-timeout 10 -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  ${EU_EVM_RPC} 2>/dev/null)

EU_HEIGHT_HEX=$(echo "$EU_RESPONSE" | jq -r '.result // empty')
EU_HEIGHT=""

if [ -n "$EU_HEIGHT_HEX" ] && [ "$EU_HEIGHT_HEX" != "null" ]; then
    # 将十六进制转换为十进制
    EU_HEIGHT=$(printf "%d" $EU_HEIGHT_HEX 2>/dev/null)
fi

# 显示结果
if [ -z "$SG_HEIGHT" ] || [ -z "$EU_HEIGHT" ]; then
    echo "错误: 无法获取块高度"
    echo ""
    echo "SG 端点响应:"
    echo "$SG_RESPONSE" | jq '.' 2>/dev/null || echo "$SG_RESPONSE"
    echo ""
    echo "EU 端点响应:"
    echo "$EU_RESPONSE" | jq '.' 2>/dev/null || echo "$EU_RESPONSE"
    exit 1
fi

echo "SG 端点块高度: $SG_HEIGHT (十六进制: $SG_HEIGHT_HEX)"
echo "EU 端点块高度: $EU_HEIGHT (十六进制: $EU_HEIGHT_HEX)"
echo ""

if [ "$SG_HEIGHT" == "$EU_HEIGHT" ]; then
    echo "✓ 块高度一致"
    exit 0
else
    DIFF=$((SG_HEIGHT - EU_HEIGHT))
    echo "✗ 块高度不一致"
    echo "  差异: $DIFF 个区块"
    if [ $DIFF -gt 0 ]; then
        echo "  SG 端点比 EU 端点高 $DIFF 个区块"
    else
        echo "  EU 端点比 SG 端点高 $((DIFF * -1)) 个区块"
    fi
    exit 1
fi

