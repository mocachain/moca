# RPC Goroutine 泄漏修复测试脚本

本目录包含用于验证 RPC goroutine 泄漏修复的测试脚本。

## 测试脚本列表

| 脚本 | 用途 | 耗时 |
|------|------|------|
| `monitor_goroutines.sh` | 基线监控 - 持续监控 goroutine 数量 | 24-48h |
| `test_rate_limit.sh` | 速率限制验证 - 验证限流器工作 | 5min |
| `test_query_timeout.sh` | 查询超时验证 - 验证超时机制 | 2min |
| `test_context_cancel.sh` | Context 取消验证 - 验证取消机制 | 1min |
| `load_test.sh` | 负载压力测试 - 验证长时间稳定性 | 1h（可配置） |
| `run_all_tests.sh` | 一键运行所有测试（除基线监控） | ~20min |

## 快速开始

### 1. 运行完整测试套件

```bash
cd /path/to/moca
./tests/rpc/run_all_tests.sh
```

### 2. 运行单个测试

```bash
# 速率限制测试
./tests/rpc/test_rate_limit.sh

# 查询超时测试
./tests/rpc/test_query_timeout.sh

# Context 取消测试
./tests/rpc/test_context_cancel.sh

# 负载测试（默认 1 小时）
./tests/rpc/load_test.sh

# 负载测试（自定义时长 10 分钟）
DURATION=600 ./tests/rpc/load_test.sh
```

### 3. 启动基线监控

```bash
# 在后台运行
nohup ./tests/rpc/monitor_goroutines.sh > /dev/null 2>&1 &
echo $! > /tmp/monitor.pid

# 查看日志
tail -f goroutine_monitor_*.log

# 停止监控
kill $(cat /tmp/monitor.pid)
```

## 测试结果判定标准

| 测试项 | 通过标准 | 失败标准 |
|--------|----------|----------|
| 基线监控 | 24h 内 < 10K，增长 < 500 | 持续增长或 > 10K |
| 速率限制 | 触发限流，服务正常 | 无限制或崩溃 |
| 查询超时 | 30s 超时，goroutine 清理 | 挂起或泄漏 |
| Context 取消 | 断开后清理，差异 < 50 | 持续累积 |
| 负载测试 | 1h 增长 < 1K，成功率 > 80% | 泄漏或不稳定 |

## 环境要求

- RPC 服务运行在 `http://127.0.0.1:8545`
- Metrics 服务运行在 `http://127.0.0.1:26660/metrics`
- 已安装 `curl`, `awk`, `bc` 等基本工具

## 注意事项

1. 在生产环境运行测试前，请先在测试环境验证
2. 负载测试会产生大量请求，注意对生产环境的影响
3. 基线监控需要运行 24-48 小时以获得准确结果
4. 所有测试日志保存在当前目录

## 相关文档

完整的分析和修复文档请参考：
See the goroutine leak analysis document.

