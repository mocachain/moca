#!/bin/bash
# 本地节点启动测试脚本
# 用于验证 moca 节点可以正常启动和出块

set -e

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
PROJECT_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)

cd "${PROJECT_ROOT}"

echo "=== 开始本地节点启动测试 ==="

# 1. 停止现有节点
echo "[1/9] 停止现有节点..."
bash ./deployment/localup/localup.sh stop || true

# 2. 清理构建产物
echo "[2/9] 清理构建产物..."
make clean

# 3. 重新构建项目
echo "[3/9] 重新构建项目..."
make build

# 4. 清理本地部署目录
echo "[4/9] 清理本地部署目录..."
rm -fr ./deployment/localup/.local

# 5. 启动本地节点（1 个 moca 验证者节点，3 个 SP 节点）
echo "[5/9] 启动本地节点（1 个 moca 验证者节点，3 个 SP 节点）..."
bash ./deployment/localup/localup.sh all 1 3

# 6. 验证节点正常运行（检查进程、日志）
echo "[6/9] 验证节点正常运行..."
sleep 5
if ! pgrep -f "mocad start" > /dev/null; then
    echo "错误: 节点进程未运行"
    exit 1
fi
echo "✓ 节点进程运行正常"

# 7. 检查节点是否正常出块
echo "[7/9] 检查节点是否正常出块..."
sleep 10

# 获取初始区块高度
INITIAL_HEIGHT=$(./build/mocad status --node tcp://127.0.0.1:26657 2>/dev/null | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*' | head -1)

if [ -z "$INITIAL_HEIGHT" ]; then
    echo "错误: 无法获取区块高度"
    exit 1
fi

echo "初始区块高度: $INITIAL_HEIGHT"

# 等待一段时间后再次检查
sleep 10
FINAL_HEIGHT=$(./build/mocad status --node tcp://127.0.0.1:26657 2>/dev/null | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*' | head -1)

if [ -z "$FINAL_HEIGHT" ]; then
    echo "错误: 无法获取最终区块高度"
    exit 1
fi

echo "最终区块高度: $FINAL_HEIGHT"

if [ "$FINAL_HEIGHT" -le "$INITIAL_HEIGHT" ]; then
    echo "错误: 区块高度未增长（初始: $INITIAL_HEIGHT, 最终: $FINAL_HEIGHT）"
    exit 1
fi

echo "✓ 节点正常出块（区块高度从 $INITIAL_HEIGHT 增长到 $FINAL_HEIGHT）"

# 检查 catching_up 状态
CATCHING_UP=$(./build/mocad status --node tcp://127.0.0.1:26657 2>/dev/null | grep -o '"catching_up":[^,}]*' | grep -o '[^:]*$')
if [ "$CATCHING_UP" = "true" ]; then
    echo "警告: 节点仍在同步中"
else
    echo "✓ 节点已同步完成（catching_up: false）"
fi

# 8. 导出存储提供者信息
echo "[8/9] 导出存储提供者信息..."
bash ./deployment/localup/localup.sh export_sps 1 3

if [ ! -f "./deployment/localup/.local/sp_export.json" ]; then
    echo "错误: SP 导出文件不存在"
    exit 1
fi
echo "✓ SP 信息导出成功"

# 9. 将 SP 信息移动到 moca-storage-provider 目录
echo "[9/9] 将 SP 信息移动到 moca-storage-provider 目录..."
mkdir -p ../moca-storage-provider/deployment/localup
mv ./deployment/localup/.local/sp_export.json ../moca-storage-provider/deployment/localup/sp.json

if [ ! -f "../moca-storage-provider/deployment/localup/sp.json" ]; then
    echo "错误: SP 文件移动失败"
    exit 1
fi
echo "✓ SP 信息已移动到 moca-storage-provider 目录"

echo ""
echo "=== 所有测试步骤完成 ==="
echo "✓ 节点启动测试通过"

