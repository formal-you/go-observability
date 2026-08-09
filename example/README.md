# 示例索引

所有命令默认从仓库根目录运行，因此相对输出路径也相对于仓库根目录。

| 示例 | 运行命令 | 说明 |
| --- | --- | --- |
| [`minimal`](minimal/) | `go run ./example/minimal` | 不依赖框架的最小 JSONL 示例 |
| [`main.go`](main.go) | `go run ./example` | Gin + telemetry + 日志出口选择 |
| [`nethttp`](nethttp/) | `go run ./example/nethttp` | 标准库 HTTP access 事件 |
| [`metrics`](metrics/) | `go run ./example/metrics` | 使用方定义 RED 与业务指标 |
| [`mall`](mall/) | `go test ./example/mall` | 领域事件名和扩展字段注册表 |
| [`samber`](samber/) | `go run ./example/samber` | 与 samber slog handler 互操作 |
| [`config`](config/) | 无 | 应用、环境变量和 Collector 配置模板 |

## 离线运行主示例

PowerShell：

```powershell
$env:OTEL_SDK_DISABLED = "true"
go run ./example
Get-Content .\logs\events.jsonl
```

bash：

```bash
OTEL_SDK_DISABLED=true go run ./example
tail -n 1 ./logs/events.jsonl
```

主示例在未设置 `OTEL_EXPORTER_OTLP_ENDPOINT` 时写 `logs/events.jsonl`。如你先 `cd example` 再运行 `go run .`，路径会变为 `example/logs/events.jsonl`；推荐始终从仓库根目录执行文档中的命令。

## 使用 OTLP

先启动参考栈，再运行主示例：

```powershell
docker compose -f .\observability\docker-compose.yml up -d
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example
```

```bash
docker compose -f ./observability/docker-compose.yml up -d
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example
```

参考栈仅用于本地开发，不是生产配置。停止命令：`docker compose -f observability/docker-compose.yml down`。
