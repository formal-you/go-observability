# 配置模板

这些文件是可复制的部署参考，库不会自动读取 YAML 或 `.env`。应用或部署平台负责加载和注入。

| 文件 | 用途 |
| --- | --- |
| [`.env.example`](.env.example) | 应用环境变量模板 |
| [`app.example.yaml`](app.example.yaml) | 应用配置结构示意，需要自行解析 |
| [`log-only.example.yaml`](log-only.example.yaml) | Log-only 预设模板，对应 `telemetry.NewLogRuntime` |
| [`otlp.example.yaml`](otlp.example.yaml) | 全 OTLP 预设模板，对应 `telemetry.NewOTLPRuntime` |
| [`all-file.example.yaml`](all-file.example.yaml) | 全文件预设模板，对应 `telemetry.NewAllFileRuntime` |
| [`file-only.example.yaml`](file-only.example.yaml) | 小单体 file-only 模板，对应 `telemetry.NewFileRuntime` |
| [`collector.example.yaml`](collector.example.yaml) | Trace、Metric、Log Collector 管线 |
| [`docker-compose.example.yml`](docker-compose.example.yml) | 应用与参考栈的 Compose 连接示意 |

完整本地栈见 [`observability`](../../observability/)；配置字段与优先级见 [配置指南](../../docs/reference/configuration.md)。复制后务必替换服务名、环境、地址和认证配置，不要把真实凭证提交到 Git。

## 从模板映射到 Runtime

库不读取 YAML；解析库由应用自行选择（viper / koanf / envconfig 等）。三份预设模板都支持：

- 只使用默认参数：调用三个快捷函数；
- 需要自定义采样、批量、队列或轮转：解析完整字段后调用 `telemetry.NewRuntime`。

### 默认参数：快捷函数

```go
// log-only.example.yaml
runtime, err := telemetry.NewLogRuntime(ctx, "order-api", telemetry.SignalOutputFile, "logs/events.jsonl")

// otlp.example.yaml
runtime, err = telemetry.NewOTLPRuntime(ctx, "order-api", "otel-collector:4317")

// all-file.example.yaml
runtime, err = telemetry.NewAllFileRuntime(ctx, "order-api", "logs")
```

### 自定义参数：NewRuntime + Config

以全 OTLP 为例，`sample_ratio`、`batch_timeout`、`export_interval`、`queue_size` 均生效：

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
    Enabled:  true,
    Endpoint: "otel-collector:4317",
    Resource: telemetry.ResourceConfig{ServiceName: "order-api"},
    Trace: telemetry.TraceConfig{
        Output:       telemetry.SignalOutputOTLP,
        SampleRatio:  1.0,
        BatchTimeout: 10 * time.Second,
    },
    Metric: telemetry.MetricConfig{
        Output:         telemetry.SignalOutputOTLP,
        ExportInterval: 30 * time.Second,
    },
    Log: telemetry.LogConfig{
        Output:       telemetry.SignalOutputOTLP,
        BatchTimeout: 2 * time.Second,
        QueueSize:    8192,
    },
})
```

Log-only 与 all-file 同理；其中 `rotation` 需解析后转为 `file.WithRotation` 并放入 `LogConfig.FileOptions`，不会由库自动读取 YAML。
## 从 app.example.yaml 解析并实例化

新增的 [`main.go`](main.go) 是一个应用层示例：它读取本目录的 `app.example.yaml`，
解析到应用自己的配置结构体，再映射为 `telemetry.Config` 和 `log.NewLogger` options。

运行方式：

```powershell
cd example/16_config
go run .
```

说明：

- 库不会自动读取 YAML；`main.go` 演示的是应用侧映射代码。
- `rotation` 先转成 `file.WithRotation`，再进入 `telemetry.LogConfig.FileOptions`。
- `sampler` 用现有 `log.NewEventTypeKeepSampler` / `log.NewResultKeepSampler` 组合，不新增 `log.Config`。
- 自定义 `sdktrace.SpanExporter` / `sdkmetric.Reader` 仍由代码注入，不放 YAML。