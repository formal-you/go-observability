# 示例索引

所有命令默认从仓库根目录运行，因此相对输出路径也相对于仓库根目录。

推荐学习顺序：`minimal` → `main.go` → `nethttp` → `blackbox` → `metrics` → `config`；
`errorhandler`、`mall`、`samber` 是专项对照示例。

| 示例 | 运行命令 | 说明 |
| --- | --- | --- |
| [`minimal`](minimal/) | `go run ./example/minimal` | 不依赖框架的最小 JSONL 示例，展示 `NewFileRuntime` 预设 |
| [`errorhandler`](errorhandler/) | `go run ./example/errorhandler` | 用失败 Writer 演示 `WithErrorHandler` 观察写入错误 |
| [`blackbox`](blackbox/) | `go run ./example/blackbox` | 真实 OTel span + Gin 请求的 JSONL/OTLP 语义黑盒测试 |
| [`main.go`](main.go) | `go run ./example` | Gin 全链路中间件（trace/access/recover/metrics/errresp），展示按信号选择输出 |
| [`nethttp`](nethttp/) | `go run ./example/nethttp` | 标准库 HTTP 全链路（trace/metrics/nethttp 收口 + access 模板） |
| [`metrics`](metrics/) | `go run ./example/metrics` | 使用方自定义业务指标，展示 `Trace=none、Metric=otlp、Log=none` 的独立装配 |
| [`mall`](mall/) | `go test ./example/mall` | 领域事件名和扩展字段注册表 |
| [`samber`](samber/) | `go run ./example/samber` | 与 samber slog handler 互操作 |
| [`config`](config/) | 无 | 应用、环境变量和 Collector 配置模板 |

## telemetry 装配方式

新 API 把配置按信号拆成 `Resource` / `Trace` / `Metric` / `Log`：

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
	Enabled:  telemetry.EnabledFromEnvironment(),
	Endpoint: telemetry.EndpointFromEnvironment(),
	Resource: telemetry.ResourceConfig{ServiceName: "order-api"},
	Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputOTLP},
	Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputOTLP},
	Log:      telemetry.LogConfig{Output: telemetry.SignalOutputOTLP},
})
restore := runtime.InstallGlobal()
defer restore()
writer, err := runtime.NewWriter(ctx) // 日志出口参数已进入 LogConfig
```

无需 Collector 的小单体用 `NewFileRuntime`，等价于 `Trace=local、Metric=none、Log=file`：

```go
runtime, err := telemetry.NewFileRuntime(telemetry.Config{
	Resource: telemetry.ResourceConfig{ServiceName: "mall-monolith"},
	Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/events.jsonl"},
})
```

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

主示例在未设置 `OTEL_EXPORTER_OTLP_ENDPOINT` 时使用 `Trace=local、Metric=none、Log=file`，写
`logs/events.jsonl`。如你先 `cd example` 再运行 `go run .`，路径会变为
`example/logs/events.jsonl`；推荐始终从仓库根目录执行文档中的命令。

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
