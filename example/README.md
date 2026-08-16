# 示例课程索引

所有命令默认从仓库根目录运行，相对输出路径也相对于仓库根目录。建议按课程顺序学习：
`01_quickstart` → `02_events` → `03_errors` → `04_sampler_masker` → `05_multiwriter` → `06_errorhandler` → `07_telemetry` → `08_http` → `09_gin` → `10_grpc` → `11_kratos` → `12_security_audit` → `13_metrics` → `14_otel_logs`；`mall` 与 `blackbox` 是端到端/验收参考，不作为第一课。

## 课程地图

| 课程 | 运行命令 | 学什么 |
| --- | --- | --- |
| [`01_quickstart`](01_quickstart/) | `go run ./example/01_quickstart` | file-only 最小事件 |
| [`02_events`](02_events/) | `go run ./example/02_events` | 六类事件与 `app.*` 扩展字段 |
| [`03_errors`](03_errors/) | `go run ./example/03_errors` | `errs` 建模与 `EventFromError` 投影 |
| [`04_sampler_masker`](04_sampler_masker/) | `go run ./example/04_sampler_masker` | Sampler 与 Masker 治理 |
| [`05_multiwriter`](05_multiwriter/) | `go run ./example/05_multiwriter` | 多出口 fan-out |
| [`06_errorhandler`](06_errorhandler/) | `go run ./example/06_errorhandler` | `WithErrorHandler` 写失败观察 |
| [`07_telemetry`](07_telemetry/) | `go run ./example/07_telemetry -mode=file` | 四种 Runtime 预设 |
| [`08_http`](08_http/) | `go run ./example/08_http` | 标准库 net/http 全链路 |
| [`09_gin`](09_gin/) | `go run ./example/09_gin` | Gin 全链路 |
| [`10_grpc`](10_grpc/) | `go run ./example/10_grpc` | gRPC Trace / Metrics 拦截器 |
| [`11_kratos`](11_kratos/) | `go run ./example/11_kratos` | kratos 错误适配 |
| [`12_security_audit`](12_security_audit/) | `go run ./example/12_security_audit` | Security / Audit 中间件 |
| [`13_metrics`](13_metrics/) | `go run ./example/13_metrics` | 使用方自定义指标 |
| [`14_otel_logs`](14_otel_logs/) | `go run ./example/14_otel_logs` | file 与 OTLP 双投影 |

## 参考样例

| 参考 | 运行命令 | 用途 |
| --- | --- | --- |
| [`mall`](mall/) | `go test ./example/mall` · `go run ./example/mall/cmd` | 领域注册表 + 端到端小商城 |
| [`blackbox`](blackbox/) | `go run ./example/blackbox -config "example/blackbox/config.example.yaml"` · `go test ./example/blackbox -v` | 正式黑盒验收样例 |
| [`16_config`](16_config/) | 无 | 应用 / 环境变量 / Collector 配置模板 |

## 三条学习路径

- **新手路径**：`01_quickstart` → `02_events` → `03_errors` → `09_gin`。
- **接错排查路径**：`06_errorhandler` → `04_sampler_masker` → `14_otel_logs` → `blackbox`。
- **架构评审路径**：`07_telemetry` → `08_http` → `10_grpc` → `11_kratos` → `12_security_audit` → `mall`。

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
writer, err := runtime.NewLogWriter(ctx) // 日志出口参数已进入 LogConfig
```

无需 Collector 的小单体用 `NewFileRuntime`，等价于 `Trace=local、Metric=none、Log=file`：

```go
runtime, err := telemetry.NewFileRuntime(telemetry.Config{
	Resource: telemetry.ResourceConfig{ServiceName: "mall-monolith"},
	Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/events.jsonl"},
})
```

## 使用 OTLP

先启动参考栈，再运行带 OTLP 的示例：

```powershell
docker compose -f .\observability\docker-compose.yml up -d
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example/09_gin
```

```bash
docker compose -f ./observability/docker-compose.yml up -d
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example/09_gin
```

参考栈仅用于本地开发，不是生产配置。停止命令：`docker compose -f observability/docker-compose.yml down`。
