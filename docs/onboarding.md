# 快速上手

## 先验证仓库

在仓库根目录运行：

```powershell
go test ./...
$env:OTEL_SDK_DISABLED = "true"
go run ./example
Get-Content .\logs\events.jsonl
```

```bash
go test ./...
OTEL_SDK_DISABLED=true go run ./example
tail -n 1 ./logs/events.jsonl
```

主示例从根目录运行时，离线 JSONL 路径是 `logs/events.jsonl`。路径按进程当前工作目录解析，不是按 `example/main.go` 所在目录解析。

## 推荐阅读顺序

1. [README](../README.md)：最小 JSONL 接入和项目边界。
2. [示例索引](../example/README.md)：选择 Gin、net/http、Metric 或 OTLP 示例。
3. [配置指南](configuration.md)：理解应用、环境变量和 Collector 三层配置。
4. [架构说明](architecture.md)：修改公共 API 前了解数据流。
5. [安全指南](security.md)：上线前完成脱敏、采样和存储治理。

## 常见修改入口

| 目标 | 位置 |
| --- | --- |
| 新增领域业务事件 | 使用方自己的包；仓库示例为 `example/mall` |
| 调整公共字段 | `keys.go`、归一化代码、测试与文档 |
| 新增写出后端 | `writer/` 下新包，实现根包 `Writer` |
| 调整三信号装配 | 公开包 `telemetry` |
| 接入 Gin | `middleware/ginlog` 与 `middleware/recover` |

修改完成后执行 `gofmt -w .`、`go vet ./...`、`go test ./...`，并把用户可见变化记录到 CHANGELOG 的 `[Unreleased]`。
