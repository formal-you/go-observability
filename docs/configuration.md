# 配置指南

配置分三层：应用代码负责事件、Logger 和 `telemetry.Config`；进程环境负责启用状态与 OTLP 地址；Collector 和存储由部署平台维护。本库不读取 YAML 配置文件，仓库中的 YAML 只作为可复制模板。

新代码使用 `telemetry.NewRuntime`，显式设置 `Config.LogOutput`，再调用 `InstallGlobal` 和 `NewWriter`。构造 Runtime 不修改进程全局 OTel 状态；应用应保存并调用恢复函数。`Setup*`、`NewLogWriter` 仅用于兼容旧接入。

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
| `Enabled` | 应用或 `EnabledFromEnvironment` 设置 | `false` 返回不含 Provider 的 Runtime |
| `LogOutput` | 无 | 必填，选择 `file` / `otlp` / `stdout` / `none` |
| `TraceSampleRatio` | `0.1` | SDK 头部采样比例，范围 `(0, 1]` |
| `TraceBatchTimeout` | `5s` | span 批量导出间隔 |
| `MetricExportInterval` | `15s` | metric 周期导出间隔 |
| `LogBatchTimeout` | `1s` | log 批量导出间隔 |
| `LogQueueSize` | `2048` | OTLP Log BatchProcessor 有界队列容量；满队列时由 OTel SDK 记录 dropped log records 诊断 |
| `TraceExporter` | 空 | 注入公开 `sdktrace.SpanExporter`，用于测试或自定义 Trace 出口；空值使用 OTLP |
| `MetricReader` | 空 | 注入公开 `sdkmetric.Reader`，用于测试或自定义 Metric 出口；空值使用 OTLP 周期 Reader |
| `Resource` | 自动构建 | 自定义 OpenTelemetry Resource |

```go
providers, err := telemetry.NewRuntime(ctx, telemetry.Config{
	Enabled:         true,
	ServiceName:     "order-api",
	ServiceVersion:  "0.1.0",
	Environment:     "production",
	LogOutput:       telemetry.LogOutputOTLP,
	TraceSampleRatio: 1.0,
})
if err != nil {
	return err
}
restore := providers.InstallGlobal()
defer restore()
defer func() {
	if err := providers.Shutdown(context.Background()); err != nil {
		slog.Error("shutdown telemetry", "err", err)
	}
}()
```

`NewRuntime` 不安装全局 Provider；`InstallGlobal` 才安装 Runtime 中非空的 Provider 和 W3C Trace Context/Baggage propagator。`ServiceName` 在 file-only 与 OTLP 模式均必填。库类型不负责监听配置变化；运行中修改需由应用重建并安全切换 Provider。

### 小单体 File-Only

不部署 Collector 的单体应用可使用 `telemetry.NewFileRuntime`。它不连接任何 OTLP
exporter，只在进程内生成合法 TraceID/SpanID，并把服务身份扁平写入每条 JSONL；完整
Trace 树不会被保存，需链路查询时再使用 OTLP/Tempo 装配。

```go
providers, err := telemetry.NewFileRuntime(telemetry.Config{
	ServiceName: "mall-monolith",
	ServiceVersion: "1.0.0",
	Environment: "production",
	Instance: "shop-server-01",
})
if err != nil {
	return err
}
defer providers.Shutdown(ctx)
restore := providers.InstallGlobal()
defer restore()
writer, err := providers.NewWriter(ctx, telemetry.WriterConfig{FilePath: "logs/events.jsonl"})
```

文件中的规范服务身份键为 `service.name`、`service.version`、`service.instance.id` 和
`deployment.environment.name`。`example/config/file-only.example.yaml` 只是配置模板，
不会被库自动读取；应用需自行解析 YAML 后映射到 `telemetry.Config`。

需要本地文件轮转时，在应用配置层读取参数并传给 file Writer：

```go
writer, err := providers.NewWriter(ctx, telemetry.WriterConfig{FilePath: "logs/events.jsonl", FileOptions: []file.Option{
	file.WithRotation(file.RotationConfig{
		MaxSizeMB: 100,
		MaxBackups: 10,
		MaxAgeDays: 30,
		Compress: true,
		LocalTime: true,
	}),
}})
```

轮转由文件大小触发；`MaxBackups=0`、`MaxAgeDays=0` 分别表示不按数量、天数清理。
应用仍需结合磁盘容量制定至少一个有限保留条件。

## 日志出口

`Runtime.NewWriter` 的选择规则：

- `LogOutputOTLP`：复用 Runtime 的 LoggerProvider，WriterConfig 必须为空。
- `LogOutputFile`：要求 `WriterConfig.FilePath`，并注入 Resource 服务身份。
- `LogOutputStdout`：使用 stdout Writer，可传 `StdoutOptions`。
- `LogOutputNone`：返回 no-op Writer，不能传其他 Writer 配置。

出口由 `Config.LogOutput` 固化；后续修改环境变量不会改变已有 Runtime，需要重新构造后再切换。

应用应检查构造错误，配置 `log.WithErrorHandler` 观察异步写入失败，并在退出时调用 `ManagedWriter.Close(ctx)`。`Runtime.NewWriter` 返回 `log.ManagedWriter`，其关闭操作幂等。需要同时写多个出口（如 stdout + 文件 + OTLP）时，用 `log.NewMultiWriter(writers...)` 组合 Writer；它会尝试关闭全部可关闭子 Writer 并聚合关闭错误，任一写入失败也不阻断其余。仅实现 `log.Writer` 的自定义 Adapter 可通过 `log.ManageWriter` 获得 no-op 关闭能力。完整代码见 [README](../README.md) 和 [`example/main.go`](../example/main.go)。

## Logger 选项

```go
logger := log.NewLogger(w,
	log.WithMinLevel(log.LevelInfo),
	log.WithTraceExtractor(otelutil.NewTraceExtractor()), // 事件未显式带 trace/span 时自动补全
	log.WithIdentityExtractor(log.ContextIdentityExtractor{}),
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
| MinLevel | 不过滤 | `WithMinLevel` 接受 DEBUG/INFO/WARN/ERROR；提高级别会同时丢弃低级别语义事件及其 AccessEvent，需明确接受关联不完整 |
| TraceExtractor | 不补全链路 | 配 `WithTraceExtractor`（如 `middleware/otelutil.NewTraceExtractor`），事件未显式带 trace/span 时自动补全 |
| Masker | 不脱敏 | 维护业务 PII 键清单并在写出前脱敏 |
| IdentityExtractor | 不补全身份 | 在认证边界注入 `IdentityContext`；可信非空值覆盖事件伪造字段 |
| BaseMetadata | 不补全 | 注入服务内稳定的公共元数据 |
| ErrorHandler | 写入错误不可见 | 接入独立、不会递归使用同一 Writer 的告警路径 |

高流量场景可显式启用以下策略：business/error/security/audit/probe 全量，access 失败高优先级保留、成功按比例保留。启用后，成功 BusinessEvent 不再保证一定存在对应 AccessEvent。

```go
log.WithSampler(log.NewEventTypeKeepSampler(
	[]log.EventType{log.EventBusiness, log.EventError, log.EventSecurity, log.EventAudit, log.EventProbe},
	log.NewResultKeepSampler(0.1),
))
```

`EventKeepSampler` 仍用于按 `order.`、`payment.` 等领域前缀保留事件；旧的类别前缀配置仅作兼容，不建议继续用于新代码。

`Subject` 输出为 `user.id` 与 `app.tenant_id`；`Actor` 输出为 `app.actor_user_id` 与
`app.actor_role`，只自动注入 Security/Audit。身份字段不进入 Metrics 维度。`FieldMasker{}`
的内置键覆盖 password、token、authorization、cookie、证件号、手机号和常见 request body；
生产接入仍必须显式配置 Masker，并补充组织特有键。

堆栈治理使用 `errs.SetStackConfig`。开发默认上限 64 KiB + full path，生产建议
`errs.ProductionStackConfig()`（16 KiB + base path）；超限记录 `app.stacktrace_truncated=true`，
panic 不允许被配置为 none。

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
