# 配置指南

配置分三层：应用代码负责事件、Logger 和 `telemetry.Config`；进程环境负责启用状态与 OTLP 地址；Collector 和存储由部署平台维护。本库不读取 YAML 配置文件，仓库中的 YAML 只作为可复制模板。

## OTel 版本边界

截至 2026-08-11，官网分别发布 OpenTelemetry Specification `1.60.0` 和 Semantic
Conventions `1.44.0`；它们与具体语言 SDK 不是同一版本线。本仓库依赖 Go OTel SDK
`v1.45.0`、Logs `v0.21.0`，代码仍明确锁定 `semconv/v1.41.0`。当前 Go 模块可导入的
semconv 最高目录为 `v1.43.0`，因此不能把官网 `1.44.0` 直接写成 Go import。
semconv 升级将作为独立变更，同步验证 schema URL、API、Resource 映射和测试。

## telemetry.Config

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `ServiceName` | 无 | 启用 telemetry 时必填，写入 `service.name` |
| `ServiceVersion` | 空 | 写入 `service.version` |
| `Environment` | `development` | 写入 `deployment.environment.name` |
| `Region` / `Instance` | 省略 | `Region` 写入低基数 `region`；`Instance` 写入 `service.instance.id` |
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

`SetupFromEnvironment` 会安装全局 Trace、Metric、Log Provider 和 W3C Trace Context/Baggage propagator。`ServiceName` 在 file-only 与 OTLP 模式均必填。库类型不负责监听配置变化；运行中修改需由应用重建并安全切换 Provider。

### 小单体 File-Only

不部署 Collector 的单体应用可使用 `telemetry.SetupFile`。它不连接任何 OTLP
exporter，只在进程内生成合法 TraceID/SpanID，并把服务身份扁平写入每条 JSONL；完整
Trace 树不会被保存，需链路查询时再使用 OTLP/Tempo 装配。

```go
providers, err := telemetry.SetupFile(telemetry.Config{
	ServiceName: "mall-monolith",
	ServiceVersion: "1.0.0",
	Environment: "production",
	Instance: "shop-server-01",
})
if err != nil {
	return err
}
defer providers.Shutdown(ctx)
writer, err := providers.NewLogWriter(ctx, "logs/events.jsonl")
```

文件中的规范服务身份键为 `service.name`、`service.version`、`service.instance.id` 和
`deployment.environment.name`。`example/config/file-only.example.json` 只是配置模板，
不会被库自动读取；应用需自行解析 JSON/YAML 后映射到 `telemetry.Config`。

## 日志出口

`providers.NewLogWriter(ctx, "logs/events.jsonl")` 的选择规则：

- `Setup` 时显式设置 `Config.Endpoint`，或 `SetupFromEnvironment` 当时读取到 `OTEL_EXPORTER_OTLP_ENDPOINT`，且 telemetry 已启用：复用 Provider 写 OTLP。
- 其他情况：写当前工作目录下的 JSONL 路径。

出口选择在 Setup 时固化；后续修改环境变量不会改变已有 `Providers` 的 endpoint 或日志出口，需要重新 Setup 后再切换。

应用应检查构造错误，配置 `log.WithErrorHandler` 观察异步写入失败，并关闭实现了 `Close(context.Context)` 的 Writer。需要同时写多个出口（如 stdout + 文件 + OTLP）时，用 `log.NewMultiWriter(writers...)` 组合 Writer，任一失败不阻断其余。完整代码见 [README](../README.md) 和 [`example/main.go`](../example/main.go)。

## Logger 选项

```go
logger := log.NewLogger(w,
	log.WithTraceExtractor(otelutil.NewTraceExtractor()), // 事件未显式带 trace/span 时自动补全
	log.WithMasker(log.FieldMasker{Keys: []string{"app.phone"}}),
	log.WithBaseMetadata(log.EventMetadata{Level: log.LevelInfo}),
	log.WithErrorHandler(func(ctx context.Context, msg string, attrs []slog.Attr, err error) {
		slog.ErrorContext(ctx, "observability write failed", "event", msg, "err", err)
	}),
)
```

| 选项 | 默认行为 | 生产建议 |
| --- | --- | --- |
| Sampler | 全量日志事件 | 保持默认可保证每个 HTTP 语义事件都有对应 AccessEvent；仅在已有网关全量 access 或明确接受关联不完整时显式采样 |
| TraceExtractor | 不补全链路 | 配 `WithTraceExtractor`（如 `middleware/trace.NewTraceExtractor`），事件未显式带 trace/span 时自动补全 |
| Masker | 不脱敏 | 维护业务 PII 键清单并在写出前脱敏 |
| BaseMetadata | 不补全 | 注入服务内稳定的公共元数据 |
| ErrorHandler | 写入错误不可见 | 接入独立、不会递归使用同一 Writer 的告警路径 |

高流量场景可显式启用以下策略：business/error/security/audit/probe 全量，access 失败恒保留、成功按比例保留。启用后，成功 BusinessEvent 不再保证一定存在对应 AccessEvent。

```go
log.WithSampler(log.NewEventKeepSampler(
	[]string{"business.", "error.", "security.", "audit.", "probe."},
	log.NewResultKeepSampler(0.1),
))
```

Gin 中间件应按 `Trace -> AccessLog -> Recover -> 其他链尾中间件` 注册。AccessLog 包在 Recover、ErrorResponse、SecurityLog、AuditLog 外层，才能在它们完成后读取最终响应状态；健康检查使用 `AccessConfig.SkipPaths` 排除。

## 头部与尾部采样

`TraceSampleRatio=0.1` 在应用 SDK 入口丢弃约 90% trace。被丢弃的数据从未导出，Collector `tail_sampling` 无法恢复。若需要按错误、延迟或属性在 Collector 侧决定保留，应用侧通常应设 `TraceSampleRatio=1.0`，再由 Collector 尾部采样；这会增加出口与 Collector 的吞吐和内存压力。

日志事件采样由 log 包 `Sampler` 控制，与 trace 采样相互独立。不要假设保留日志就一定能查询到对应 trace。

## 部署配置

- 环境变量模板：[`example/config/.env.example`](../example/config/.env.example)
- 应用配置结构示意：[`example/config/app.example.yaml`](../example/config/app.example.yaml)
- Collector 示例：[`example/config/collector.example.yaml`](../example/config/collector.example.yaml)
- 本地参考栈：[`observability`](../observability/)
- 分信号模板：[`observability/templates`](../observability/templates/)
