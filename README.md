# go-observability

> 开发规则见 [AGENTS.md](AGENTS.md)；防漂移硬规则（EventName 常量、semconv 1.41.0 键名、双投影）以 AGENTS.md 为准。

基于 OpenTelemetry 语义约定的 Go 语义化日志组件（源即规范，方案2）。

模块路径：`github.com/formal-you/go-observability`（**发布前请确认**为正式 GitHub 地址后再打 tag）。

- **Go**：1.25+（见 `go.mod`）
- **语言**：导出注释与文档以**中文**为主（C4）；术语保留英文（span / trace / semconv）
- **版本**：v0.x 允许破坏性变更，见 [CHANGELOG.md](CHANGELOG.md)

## 新人引导

新接手先读 [docs/onboarding.md](docs/onboarding.md)（定位 / 阅读顺序 / 15 分钟上手）；
代码走读看 [docs/architecture.md](docs/architecture.md)，流程与规则看 [docs/workflow.md](docs/workflow.md)。

## 特性

- 核心包零外部依赖（仅标准库：log/slog、net、time、fmt、strings），属性键直接对齐 OTel semconv 1.41.0 + app.* vendor 命名空间。
- 三信号装配（internal/telemetry）：Trace / Metric / Log provider + OTLP gRPC 导出 + A3 采样/频率 + A7 资源属性；env 控制（OTEL_SDK_DISABLED / OTEL_EXPORTER_OTLP_ENDPOINT / GO_OBSERVABILITY_REGION / GO_OBSERVABILITY_INSTANCE）；出口收敛 SetupFromEnvironment + NewLogWriter（B9：endpoint env 空→JSONL、非空→OTLP）。
- 六类事件：access / business / error / audit / security / probe；`BusinessPayload.ExtraAttrs` 承载接入方领域键。核心只登记框架级 EventName；领域 business.* 见 [`example/mall`](example/mall)（C2）。
- EventPayload + Logger/Writer：可选 `WithSampler` / `WithMasker`；内置 `ResultKeepSampler`（高价值 result 必留）与 `FieldMasker`（密钥类键脱敏，C6）。
- Writer：OTLP / stdout / file(JSONL)。
- **中间件（C3）**：官方仅 Gin（`middleware/ginlog` + `recover`）；`net/http` 见 [`example/nethttp`](example/nethttp)。不 import middleware 即可纯用核心 log。
- 错误体系（errs）：ErrorKind / ErrorType v1 受控枚举 / AppError 接口 / BizError+SystemError / StackRule（error.type 前缀 → 必记/可选/不记堆栈），零外部依赖；根包 log.EventFromError 一键投影为 business/error 事件，缺省日志级别由 log.LevelOf 规则表推导（B3 定稿：validation/business=WARN、system 重试中=WARN/耗尽=ERROR，显式 Level 优先）。
- panic 收口中间件（middleware/recover）：recover → runtime.panic ErrorEvent（必记堆栈）+ 统一 500，trace/span 自动填充。
- 采集频率不在核心：每次 Emit 同步写，频率由 opentelemetry-collector batch processor 配置决定。

## 快速开始

```go
import (
    "github.com/gin-gonic/gin"
    "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

    "github.com/formal-you/go-observability"
    "github.com/formal-you/go-observability/middleware/ginlog"
    "github.com/formal-you/go-observability/writer/file"
)

w, _ := file.New("logs/events.jsonl") // 或 otlp.New(ctx)、stdout.New(ctx)
logger := log.NewLogger(w,
    log.WithSampler(log.ResultKeepSampler{Ratio: 1}), // 生产可按流量调低
    log.WithMasker(log.FieldMasker{}),                // 可 Keys 追加 PII 键
)

r := gin.New()
r.Use(otelgin.Middleware("my-service"))
r.Use(ginlog.Middleware(ginlog.Config{Logger: logger}))
```

业务事件：

```go
logger.Emit(ctx, log.BusinessEvent{
    EventMetadata: log.EventMetadata{Level: log.LevelInfo, TraceID: traceID, SpanID: spanID},
    Data: log.BusinessPayload{
        EventName: log.NewEventName("business", "order", "paid"), // 或接入方注册表常量
        Result:    log.ResultSuccess,
    },
})
```

领域 10 事件 + ExtraAttrs 键示范：[`example/mall`](example/mall)。

## 包布局

```
（模块根 = 包 log）
log/                   核心：types/keys/metadata/payload/events/normalize/log（Logger/Writer）
middleware/ginlog/     Gin 中间件（otelgin trace 提取）
middleware/recover/   panic 收口中间件（runtime.panic ErrorEvent + 统一 500）
errs/                错误体系：Kind/Type v1/AppError/BizError/SystemError/StackRule（零外部依赖）
writer/otlp/           OTLP Logs Writer（otlploggrpc + sdk/log）
writer/stdout/         stdout Writer（stdoutlog）
writer/file/           JSONL 文件 Writer
internal/attrkv/       slog.Attr → OTel KeyValue / Severity 映射（内部共享）
internal/telemetry/   三信号装配（Resource + Trace/Metric/Log provider，A3 采样频率 + A7 资源属性）
example/               演示（默认落盘 example/logs/events.jsonl）
example/metrics/       使用方自建指标（B5：Meter + PromQL 提示）
example/mall/          接入方业务事件注册表（C2/B4）
example/nethttp/       无 Gin 的 access 埋点（C3）
example/metrics/       使用方 Meter（B5）
```

## OTel 符合性

- 属性键直接用 semconv 名（http.request.method、error.type、exception.stacktrace 等）+ app.* vendor 命名空间。
- trace_id/span_id：OTLP 路径由 ctx 的 span context 自动关联到 LogRecord（Writer 不写属性）；ginlog 默认从 otelgin span 提取并填充 EventMetadata，供 file/stdout 扁平列使用。
- request_id：显式值优先（如网关 X-Request-ID），为空且 trace_id 非空时由归一化层自动派生 trace_id 前缀（12 hex，A4 免映射表约定）；file/stdout 扁平列保留。
- service/version/env/instance 由 SDK Resource 提供（见 example 的 setupTracer）。
- 详见 方案2-直接符合规范.md。

## Observability 参考栈（observability/）

本地可运行 LGTM 栈（Tempo + Loki + Mimir + Grafana + OTel Collector-contrib），
Trace/Metric/Log 三条 OTLP 管线、Grafana datasource 与 go-observability-overview
联动面板（log→trace / metric→trace）已预置。快速启动、健康检查、验证步骤与
"待你确定清单"见 observability/README.md。

## 生态对比（samber）

samber 的 slog 系列覆盖了我们的大部分零件（fanout/PII/采样/各后端 handler/框架中间件），但没有类型化事件 + semconv 语义治理 + 双投影这一层。详见 [docs/samber-comparison.md](docs/samber-comparison.md)。

## 验证

- `go test ./...` 全过：核心 20 + errs 14 + ginlog 6 + recover 3 + telemetry 8 + attrkv 4 + file 1 + stdout 2 + otlp 3。
- 端到端：example 起 Gin → 请求 → access 事件落盘 example/logs/events.jsonl，字段含 semconv 键、trace_id/span_id、level/result 映射（200→INFO/success，404→WARN/failed，503→WARN/failed）。

## Metric 立场

本库 **不** 内置业务/RED 指标注册表。使用方 `providers.Meter(name)` 自建。  
见 `observability/templates/metric-*.example.*`、[`example/metrics`](example/metrics)。

## 安全提示

- `FieldMasker` 默认只盖密钥类键，**不是**完整合规方案；生产请按 PII 清单追加 `Keys` 或自实现 `Masker`。
- 默认 trace 采样率 0.1 为演示值（`telemetry.Config.TraceSampleRatio`），生产按流量调整。
- 本库不是完整 APM；告警/保留期/容量由接入方 runbook 负责。

## Roadmap

- [x] OTLP Writer / telemetry 三信号 / 配置模板
- [x] ResultKeepSampler + FieldMasker（C6）
- [x] CI（fmt/vet/test/race/govulncheck）
- [ ] Grafana 面板精细化 + collector tail_sampling 判定属性
- [ ] 字段映射表导出 Collector transform
- [ ] 运营宽表（ClickHouse，接入方）

## License

MIT · 见 [LICENSE](LICENSE) · 贡献见 [CONTRIBUTING.md](CONTRIBUTING.md)
