# 配置指南

配置分三层：应用代码负责事件、Logger 和 `telemetry.Config`；进程环境负责启用状态与 OTLP 地址；Collector 和存储由部署平台维护。本库不读取 YAML 配置文件，仓库中的 YAML 只作为可复制模板。

`telemetry.Config` 按信号拆为 `Resource` / `Trace` / `Metric` / `Log`。应用先 `NewRuntime`，再显式调用 `InstallGlobal`；日志 Writer 通过 `Runtime.NewWriter(ctx)` 创建，参数已进入 `Config.Log`。构造 Runtime 不修改进程全局 OTel 状态；应用应保存并调用恢复函数。

## OTel 版本边界

截至 2026-08-11，官网分别发布 OpenTelemetry Specification `1.60.0` 和 Semantic
Conventions `1.44.0`；它们与具体语言 SDK 不是同一版本线。本仓库依赖 Go OTel SDK
`v1.45.0`、Logs `v0.21.0`，代码仍明确锁定 `semconv/v1.41.0`。当前 Go 模块可导入的
semconv 最高目录为 `v1.43.0`，因此不能把官网 `1.44.0` 直接写成 Go import。
当前锁定 `v1.41.0` 是既有契约，不是漏更新：`v1.43.0` 已可导入，但升级会改变 schema URL、Resource 映射和测试期望，因此标记为“待升级”，不与本轮配置重构合并。semconv 升级将作为独立变更，同步验证 schema URL、API、Resource 映射和测试。

## telemetry.Config

### 顶层

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `Enabled` | 应用或 `EnabledFromEnvironment` 设置 | `false` 返回不含 Provider 的 Runtime；`Log.Output` 为空或 `otlp` 时归一化为 `file` |
| `Endpoint` | `127.0.0.1:4317` | OTLP gRPC endpoint，可用 `host:port` 或 URL；仅在有信号选择 OTLP 时解析 |

`Config.Endpoint` 只描述出口地址，不包含 TLS/mTLS、证书或认证头。生产环境连接非本地
Collector 时，必须另行配置传输安全和认证；当前 `telemetry.Config` 不暴露这些能力，
需要通过注入 Trace/Metric 的 `Exporter`/`Reader` 或自建 Log Provider 完成，不能把
“配置了 Endpoint”误认为已完成安全链路。

### ResourceConfig

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `ServiceName` | 无 | 启用 telemetry 时必填，写入 `service.name` |
| `ServiceVersion` | 空 | 写入 `service.version` |
| `Environment` | `development` | 写入 `deployment.environment.name` |
| `Region` / `Instance` | 省略 | `Region` 写入低基数 `region`；`Instance` 写入 `service.instance.id` |
| `Override` | 空 | 自定义 OpenTelemetry Resource；非空时由调用方负责属性完整性 |

### TraceConfig

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `Output` | `otlp` | `otlp` / `file` / `stdout` / `none` / `local`；`local` 只生成合法 TraceID/SpanID 不导出完整 Span |
| `FilePath` | 空 | `Output=file` 时必填 |
| `SampleRatio` | `0.1` | SDK 头部采样比例，范围 `(0, 1]`；根节点决策经 W3C `traceparent` 传播，下游尊重父级，不重复掷骰 |
| `BatchTimeout` | `5s` | span 批量导出间隔 |
| `Exporter` | 空 | 注入公开 `sdktrace.SpanExporter`，仅 `Output=otlp` 可用 |

### MetricConfig

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `Output` | `otlp` | `otlp` / `file` / `stdout` / `none` |
| `FilePath` | 空 | `Output=file` 时必填 |
| `ExportInterval` | `15s` | metric 周期导出间隔 |
| `Reader` | 空 | 注入公开 `sdkmetric.Reader`，仅 `Output=otlp` 可用 |

### LogConfig

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `Output` | 无 | 必填，选择 `file` / `otlp` / `stdout` / `none` |
| `FilePath` | 空 | `Output=file` 时必填 |
| `FileOptions` | 空 | 仅 `Output=file` |
| `StdoutOptions` | 空 | 仅 `Output=stdout` |
| `BatchTimeout` | `1s` | log 批量导出间隔 |
| `QueueSize` | `2048` | OTLP Log BatchProcessor 有界队列容量；满队列时由 OTel SDK 内部日志记录 `dropped log records`，本库 `Runtime.Stats` 不统计该值 |

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
	Enabled:  true,
	Endpoint: "127.0.0.1:4317",
	Resource: telemetry.ResourceConfig{
		ServiceName:    "order-api",
		ServiceVersion: "0.1.0",
		Environment:    "production",
	},
	Trace: telemetry.TraceConfig{
		Output:      telemetry.SignalOutputOTLP,
		SampleRatio: 1.0,
	},
	Metric: telemetry.MetricConfig{
		Output: telemetry.SignalOutputOTLP,
	},
	Log: telemetry.LogConfig{
		Output: telemetry.SignalOutputOTLP,
	},
})
if err != nil {
	return err
}
restore := runtime.InstallGlobal()
defer restore()
defer func() {
	if err := runtime.Shutdown(context.Background()); err != nil {
		slog.Error("shutdown telemetry", "err", err)
	}
}()

writer, err := runtime.NewWriter(ctx)
if err != nil {
	return err
}
defer writer.Close(ctx)
```

`NewRuntime` 不安装全局 Provider；`InstallGlobal` 才安装 Runtime 中非空的 Provider 和 W3C Trace Context/Baggage propagator。Trace 采样器使用 `ParentBased(TraceIDRatioBased(SampleRatio))`，根节点做出的采样决定会随 `traceparent` 沿调用链传播，下游服务尊重父级决定，不重新独立采样。`Resource.ServiceName` 在 file-only 与 OTLP 模式均必填。库类型不负责监听配置变化；运行中修改需由应用重建并安全切换 Provider。

### 出口组合速查表

选择出口时，`Enabled` 是总开关；`Enabled=true` 时 `Resource.ServiceName` 和
`Log.Output` 必填，`Trace.Output` / `Metric.Output` 为空时默认 `otlp`。构造后的统一
使用方式为 `NewRuntime` → `InstallGlobal` → `NewWriter` → `Shutdown`，以下示例只展示
`Config` 差异。

```text
Config
├─ Enabled=false
│   └─ Log.Output: ""/otlp→file(需 FilePath) | file(需 FilePath) | stdout | none
└─ Enabled=true
    ├─ Trace.Output:  otlp | file(需 FilePath) | stdout | none | local
    ├─ Metric.Output: otlp | file(需 FilePath) | stdout | none
    └─ Log.Output:    file(需 FilePath) | otlp | stdout | none
```

| 场景 | Enabled | Trace | Metric | Log | 说明 |
| --- | --- | --- | --- | --- | --- |
| 生产全链路 | true | otlp | otlp | otlp | 三信号发 Collector，需 `Endpoint` |
| 本地开发 | true | stdout | stdout | stdout | 三信号全打 stdout |
| 混合出口 | true | otlp | file | file | Trace 进 Collector，Metric/Log 落本地 |
| file-only 小单体 | true | local | none | file | 用 `NewFileRuntime` |
| 只写本地日志 | false | 忽略 | 忽略 | file/stdout/none | 不创建 Provider，只保留 Log Writer |
| 完全关闭 | false | 忽略 | 忽略 | none | 不采集、不写日志 |

三个默认档位已提供快捷函数，只需必填参数；配置文件见 [`example/config/`](../example/config/README.md)：

| 快捷函数 | 签名 | 配置模板 |
| --- | --- | --- |
| `NewLogRuntime` | `(ctx, serviceName, output, filePath)` | [`log-only.example.yaml`](../example/config/log-only.example.yaml) |
| `NewOTLPRuntime` | `(ctx, serviceName, endpoint)` | [`otlp.example.yaml`](../example/config/otlp.example.yaml) |
| `NewAllFileRuntime` | `(ctx, serviceName, dir)` | [`all-file.example.yaml`](../example/config/all-file.example.yaml) |

```go
// 1. 只开 Log：output 为 file 或 stdout
runtime, err := telemetry.NewLogRuntime(ctx, "order-api", telemetry.SignalOutputFile, "logs/events.jsonl")

// 2. 全 OTLP：endpoint 指向 Collector（k8s 中为 Collector Service/DaemonSet）
runtime, err = telemetry.NewOTLPRuntime(ctx, "order-api", "otel-collector:4317")

// 3. 全 file：在 dir 下生成 events.jsonl / trace.jsonl / metric.jsonl
runtime, err = telemetry.NewAllFileRuntime(ctx, "order-api", "logs")
```

**生产全链路（全 OTLP）**

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
    Enabled:  true,
    Endpoint: "127.0.0.1:4317",
    Resource: telemetry.ResourceConfig{ServiceName: "order-api"},
    Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputOTLP},
    Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputOTLP},
    Log:      telemetry.LogConfig{Output: telemetry.SignalOutputOTLP},
})
```

**本地开发（全 stdout）**

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
    Enabled:  true,
    Resource: telemetry.ResourceConfig{ServiceName: "order-api"},
    Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputStdout},
    Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputStdout},
    Log:      telemetry.LogConfig{Output: telemetry.SignalOutputStdout},
})
```

**混合出口（Trace OTLP + Metric/Log 文件）**

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
    Enabled:  true,
    Endpoint: "127.0.0.1:4317",
    Resource: telemetry.ResourceConfig{ServiceName: "order-api"},
    Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputOTLP},
    Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/metrics.out"},
    Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/events.jsonl"},
})
```

**file-only 小单体**

```go
runtime, err := telemetry.NewFileRuntime(telemetry.Config{
    Resource: telemetry.ResourceConfig{ServiceName: "mall-monolith"},
    Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/events.jsonl"},
})
```

**只写本地日志（不创建 Provider）**

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
    Enabled: false,
    Log:     telemetry.LogConfig{Output: telemetry.SignalOutputStdout},
})
```

**完全关闭**

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
    Enabled: false,
    Log:     telemetry.LogConfig{Output: telemetry.SignalOutputNone},
})
```

### 小单体 File-Only

不部署 Collector 的单体应用可使用 `telemetry.NewFileRuntime`。它等价于
`Trace=local、Metric=none、Log=file`：不连接任何 OTLP exporter，只在进程内生成合法
TraceID/SpanID，并把服务身份扁平写入每条 JSONL；完整 Trace 树不会被保存，需链路查询
时再使用 OTLP/Tempo 装配。

```go
runtime, err := telemetry.NewFileRuntime(telemetry.Config{
	Resource: telemetry.ResourceConfig{
		ServiceName:    "mall-monolith",
		ServiceVersion: "1.0.0",
		Environment:    "production",
		Instance:       "shop-server-01",
	},
	Log: telemetry.LogConfig{
		Output:   telemetry.SignalOutputFile,
		FilePath: "logs/events.jsonl",
	},
})
if err != nil {
	return err
}
defer runtime.Shutdown(ctx)
restore := runtime.InstallGlobal()
defer restore()
writer, err := runtime.NewWriter(ctx)
```

文件中的规范服务身份键为 `service.name`、`service.version`、`service.instance.id` 和
`deployment.environment.name`。`example/config/file-only.example.yaml` 只是配置模板，
不会被库自动读取；应用需自行解析 YAML 后映射到 `telemetry.Config`。

需要本地文件轮转时，把选项写入 `LogConfig.FileOptions`：

```go
runtime, err := telemetry.NewFileRuntime(telemetry.Config{
	Resource: telemetry.ResourceConfig{ServiceName: "mall-monolith"},
	Log: telemetry.LogConfig{
		Output:   telemetry.SignalOutputFile,
		FilePath: "logs/events.jsonl",
		FileOptions: []file.Option{
			file.WithRotation(file.RotationConfig{
				MaxSizeMB: 100,
				MaxBackups: 10,
				MaxAgeDays: 30,
				Compress:   true,
				LocalTime:  true,
			}),
		},
	},
})
```

轮转由文件大小触发；`MaxBackups=0`、`MaxAgeDays=0` 分别表示不按数量、天数清理。
两者同时为 `0` 时不会清理旧文件，库会在创建 Writer 时输出 WARN 自诊断，但不会强制拒绝；生产环境仍应由应用结合磁盘容量制定至少一个有限保留条件。

## 日志出口

`Runtime.NewWriter(ctx)` 根据 `Config.Log.Output` 创建 Writer：

- `SignalOutputOTLP`：复用 Runtime 的 LoggerProvider，Writer 不拥有也不会关闭 Provider。
- `SignalOutputFile`：要求 `Log.FilePath`，并注入 Resource 服务身份。
- `SignalOutputStdout`：使用 stdout Writer，可传 `Log.StdoutOptions`。
- `SignalOutputNone`：返回 no-op Writer。

出口由 `Config.Log.Output` 固化；后续修改环境变量不会改变已有 Runtime，需要重新构造后再切换。

应用应检查构造错误，配置 `log.WithErrorHandler` 观察异步写入失败，并在退出时调用 `ManagedWriter.Close(ctx)`。`Runtime.NewWriter` 返回 `log.ManagedWriter`，其关闭操作幂等。需要同时写多个出口（如 stdout + 文件 + OTLP）时，用 `log.NewMultiWriter(writers...)` 组合 Writer；写入阶段任一子 Writer 失败不阻断其余，最终用 `errors.Join` 聚合错误；关闭阶段同样尝试关闭全部可关闭子 Writer 并聚合关闭错误。仅实现 `log.Writer` 的自定义 Adapter 可通过 `log.ManageWriter` 获得 no-op 关闭能力。完整代码见 [README](../README.md) 和 [`example/main.go`](../example/main.go)。

## OTLP 队列溢出与告警

`Log.QueueSize` 达到上限时，OTel Log SDK 会丢弃最旧 LogRecord，并在 SDK 内部日志输出
`dropped log records`。默认内部日志只显示 Error 级别，这条 Warn 通常不可见；本库的
`Runtime.Stats` 只统计 exporter 返回的导出错误，不统计队列溢出。生产接入至少做两件事：

1. 在启动早期配置能显示 SDK 内部 Warn 的 `logr.Logger`，避免 `dropped log records`
   静默丢失。OTel Go SDK 默认只显示 Error，需要把内部 logger verbosity 调到至少 1：

   ```go
   import (
       "log"
       "os"

       "github.com/go-logr/stdr"
       "go.opentelemetry.io/otel"
   )

   logger := stdr.New(log.New(os.Stderr, "", log.LstdFlags))
   stdr.SetVerbosity(1) // 显示 SDK 内部 Warn，例如 dropped log records
   otel.SetLogger(logger)
   ```

2. 把队列容量与导出速率作为容量指标监控；持续出现 drop 时优先提高 `QueueSize`、
   调优导出间隔，或对日志事件启用采样，而不是只增大队列掩盖背压。

`log.WithErrorHandler` 只观察 Writer 写入失败，看不到 SDK 队列丢弃，不要用它替代上述监控。

## Logger 选项

```go
logger := log.NewLogger(w,
	log.WithMinLevel(log.LevelInfo),
	log.WithTraceExtractor(otelutil.NewTraceExtractor()), // 事件未显式带 trace/span 时自动补全
	log.WithIdentityExtractor(log.ContextIdentityExtractor{}),
	log.WithMasker(log.FieldMasker{Keys: []string{"app.phone"}}),
	log.WithBaseMetadata(log.EventMetadata{Level: log.LevelInfo}),
	log.WithErrorHandler(func(ctx context.Context, eventType string, attrs []slog.Attr, err error) {
		slog.ErrorContext(ctx, "observability write failed", "event", eventType, "err", err)
	}),
)
```

| 选项 | 默认行为 | 生产建议 |
| --- | --- | --- |
| Sampler | 全量日志事件 | 保持默认可保证每个 HTTP 语义事件都有对应 AccessEvent；仅在已有网关全量 access 或明确接受关联不完整时显式采样 |
| MinLevel | 不过滤 | `WithMinLevel` 接受 DEBUG/INFO/WARN/ERROR；提高级别会同时丢弃低级别语义事件及其 AccessEvent，需明确接受关联不完整 |
| TraceExtractor | 不补全链路 | 配 `WithTraceExtractor`（如 `middleware/otelutil.NewTraceExtractor`），事件未显式带 trace/span 时自动补全 |
| Masker | 不脱敏 | 只作用于日志事件 attrs；维护业务 PII 键清单并在写出前脱敏。Trace span attributes / Metric labels 不在其拦截范围内，需在生成 span/指标前自行脱敏或另建统一拦截点 |
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
生产接入仍必须显式配置 Masker，并补充组织特有键。`Masker` 是日志管线的钩子，不覆盖 Trace/Metric 属性；同一敏感字段同时进入 span 或 metric label 时不会因配置了 `WithMasker` 而自动脱敏。

堆栈治理使用 `errs.SetStackConfig`。开发默认上限 64 KiB + full path，生产建议
`errs.ProductionStackConfig()`（16 KiB + base path）；超限记录 `app.stacktrace_truncated=true`，
panic 不允许被配置为 none。

Gin 中间件应按 `Trace -> AccessLog -> Recover -> 其他链尾中间件` 注册。AccessLog 包在 Recover、ErrorResponse、SecurityLog、AuditLog 外层，才能在它们完成后读取最终响应状态；健康检查使用 `AccessConfig.SkipPaths` 排除。

## 头部与尾部采样

`Trace.SampleRatio=0.1` 在应用 SDK 入口丢弃约 90% trace。被丢弃的数据从未导出，Collector `tail_sampling` 无法恢复。若需要按错误、延迟或属性在 Collector 侧决定保留，应用侧通常应设 `Trace.SampleRatio=1.0`，再由 Collector 尾部采样；这会增加出口与 Collector 的吞吐和内存压力。

日志事件采样由 log 包 `Sampler` 控制，与 trace 采样相互独立。不要假设保留日志就一定能查询到对应 trace。

## 部署配置

- 环境变量模板：[`example/config/.env.example`](../example/config/.env.example)
- 应用配置结构示意：[`example/config/app.example.yaml`](../example/config/app.example.yaml)
- Collector 示例：[`example/config/collector.example.yaml`](../example/config/collector.example.yaml)
- 本地参考栈：[`observability`](../observability/)
- 分信号模板：[`observability/templates`](../observability/templates/)
