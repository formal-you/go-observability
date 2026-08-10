# 配置指南

配置分三层：应用代码负责事件、Logger 和 `telemetry.Config`；进程环境负责启用状态与 OTLP 地址；Collector 和存储由部署平台维护。本库不读取 YAML 配置文件，仓库中的 YAML 只作为可复制模板。

## telemetry.Config

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `ServiceName` | 无 | 启用 telemetry 时必填，写入 `service.name` |
| `ServiceVersion` | 空 | 写入 `service.version` |
| `Environment` | `dev` | 写入 `deployment.environment` |
| `Region` / `Instance` | 省略 | 可选低基数资源属性 |
| `Endpoint` | 环境变量或 `127.0.0.1:4317` | OTLP gRPC endpoint，可用 `host:port` 或 URL |
| `Enabled` | `SetupFromEnvironment` 从环境读取 | `false` 返回空 Providers |
| `TraceSampleRatio` | `0.1` | SDK 头部采样比例，范围 `(0, 1]` |
| `TraceBatchTimeout` | `5s` | span 批量导出间隔 |
| `MetricExportInterval` | `15s` | metric 周期导出间隔 |
| `LogBatchTimeout` | `1s` | log 批量导出间隔 |
| `Resource` | 自动构建 | 自定义 OpenTelemetry Resource |

```go
providers, err := telemetry.SetupFromEnvironment(ctx, telemetry.Config{
	ServiceName:     "order-api",
	ServiceVersion:  "0.1.0",
	Environment:     "production",
	TraceSampleRatio: 1.0,
})
if err != nil {
	return err
}
defer func() {
	if err := providers.Shutdown(context.Background()); err != nil {
		slog.Error("shutdown telemetry", "err", err)
	}
}()
```

`SetupFromEnvironment` 会安装全局 Trace、Metric、Log Provider 和 W3C Trace Context/Baggage propagator。库类型不负责监听配置变化；运行中修改需由应用重建并安全切换 Provider。

## 日志出口

`providers.NewLogWriter(ctx, "logs/events.jsonl")` 的选择规则：

- `Setup` 时显式设置 `Config.Endpoint`，或 `SetupFromEnvironment` 当时读取到 `OTEL_EXPORTER_OTLP_ENDPOINT`，且 telemetry 已启用：复用 Provider 写 OTLP。
- 其他情况：写当前工作目录下的 JSONL 路径。

出口选择在 Setup 时固化；后续修改环境变量不会改变已有 `Providers` 的 endpoint 或日志出口，需要重新 Setup 后再切换。

应用应检查构造错误，配置 `log.WithErrorHandler` 观察异步写入失败，并关闭实现了 `Close(context.Context)` 的 Writer。需要同时写多个出口（如 stdout + 文件 + OTLP）时，用 `log.NewMultiWriter(writers...)` 组合 Writer，任一失败不阻断其余。完整代码见 [README](../README.md) 和 [`example/main.go`](../example/main.go)。

## Logger 选项

```go
logger := log.NewLogger(w,
	log.WithSampler(log.NewEventKeepSampler(
		[]string{"business.", "error.", "security.", "audit.", "probe."},
		log.NewResultKeepSampler(0.1), // access 成功按比例采样，失败恒保留
	)),
	log.WithTraceExtractor(tracemw.NewTraceExtractor()), // 事件未显式带 trace/span 时自动补全（middleware/trace）
	log.WithMasker(log.FieldMasker{Keys: []string{"app.phone"}}),
	log.WithBaseMetadata(log.EventMetadata{Level: log.LevelInfo}),
	log.WithErrorHandler(func(ctx context.Context, msg string, attrs []slog.Attr, err error) {
		slog.ErrorContext(ctx, "observability write failed", "event", msg, "err", err)
	}),
)
```

| 选项 | 默认行为 | 生产建议 |
| --- | --- | --- |
| Sampler | 全量日志事件 | 推荐 `NewEventKeepSampler`：业务/错误/安全/审计/探测全量，access 成功按比例采样、失败恒保留（非法入参构造期 panic） |
| TraceExtractor | 不补全链路 | 配 `WithTraceExtractor`（如 `middleware/trace.NewTraceExtractor`），事件未显式带 trace/span 时自动补全 |
| Masker | 不脱敏 | 维护业务 PII 键清单并在写出前脱敏 |
| BaseMetadata | 不补全 | 注入服务内稳定的公共元数据 |
| ErrorHandler | 写入错误不可见 | 接入独立、不会递归使用同一 Writer 的告警路径 |

## 头部与尾部采样

`TraceSampleRatio=0.1` 在应用 SDK 入口丢弃约 90% trace。被丢弃的数据从未导出，Collector `tail_sampling` 无法恢复。若需要按错误、延迟或属性在 Collector 侧决定保留，应用侧通常应设 `TraceSampleRatio=1.0`，再由 Collector 尾部采样；这会增加出口与 Collector 的吞吐和内存压力。

日志事件采样由根包 `Sampler` 控制，与 trace 采样相互独立。不要假设保留日志就一定能查询到对应 trace。

## 部署配置

- 环境变量模板：[`example/config/.env.example`](../example/config/.env.example)
- 应用配置结构示意：[`example/config/app.example.yaml`](../example/config/app.example.yaml)
- Collector 示例：[`example/config/collector.example.yaml`](../example/config/collector.example.yaml)
- 本地参考栈：[`observability`](../observability/)
- 分信号模板：[`observability/templates`](../observability/templates/)
