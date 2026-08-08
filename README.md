# go-observability

基于 OpenTelemetry 语义约定的 Go 语义化日志组件（源即规范，方案2）。

模块路径：`github.com/formal-you/go-observability`（发布前请确认为正式仓库地址）。

## 特性

- 核心包零外部依赖（仅 log/slog、net、time），属性键直接对齐 OTel semconv + mall.* vendor 命名空间。
- 六类事件：access / business / error / audit / security / probe，每类有具体事件结构体（领域构造、中间件类型断言）。
- EventPayload + Logger/Writer 抽象：采样、脱敏、后端可注入替换。
- Writer 实现：OTLP（otlploggrpc）、stdout（stdoutlog）、file（JSONL 落盘）。
- Gin 中间件（middleware/ginlog）开箱即用，自动从 otelgin span 提取 trace_id/span_id。
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
logger := log.NewLogger(w)

r := gin.New()
r.Use(otelgin.Middleware("my-service")) // 生成 OTel span
r.Use(ginlog.Middleware(ginlog.Config{Logger: logger}))
```

业务事件：

```go
logger.Emit(ctx, log.BusinessEvent{
    EventMetadata: log.EventMetadata{Level: log.LevelInfo, TraceID: traceID, SpanID: spanID},
    Data: log.BusinessPayload{EventName: "business.order.paid", BusinessCode: "ORD-200", Result: log.ResultSuccess},
})
```

## 包布局

```
（模块根 = 包 log）
log/                   核心：types/keys/metadata/payload/events/normalize/log（Logger/Writer）
middleware/ginlog/     Gin 中间件（otelgin trace 提取）
writer/otlp/           OTLP Logs Writer（otlploggrpc + sdk/log）
writer/stdout/         stdout Writer（stdoutlog）
writer/file/           JSONL 文件 Writer
internal/attrkv/       slog.Attr → OTel KeyValue / Severity 映射（内部共享）
example/               演示（默认落盘 example/logs/events.jsonl）
```

## OTel 符合性

- 属性键直接用 semconv 名（http.request.method、error.type、exception.stacktrace 等）+ mall.* vendor 命名空间。
- trace_id/span_id 由调用方从 span context 填充进 EventMetadata；ginlog 默认从 otelgin span 提取；Writer 层转 OTLP TraceId/SpanId 字节。
- service/version/env/instance 由 SDK Resource 提供（见 example 的 setupTracer）。
- 详见 方案2-直接符合规范.md。

## 生态对比（samber）

samber 的 slog 系列覆盖了我们的大部分零件（fanout/PII/采样/各后端 handler/框架中间件），但没有类型化事件 + semconv 语义治理 + 双投影这一层。详见 [docs/samber-comparison.md](docs/samber-comparison.md)。

## 验证

- `go test ./...` 全过：核心 4 + ginlog 4 + attrkv 2 + file 1 + stdout 1 + otlp 冒烟 1。
- 端到端：example 起 Gin → 请求 → access 事件落盘 example/logs/events.jsonl，字段含 semconv 键、trace_id/span_id、level/result 映射（200→INFO/success，404→WARN/failed）。

## Roadmap

- [x] OTLP Writer（otlploggrpc）
- [ ] Masker / Sampler 默认实现（按 Result 强制保留）
- [ ] 字段映射表导出（Go 常量 → Collector transform 配置）
- [ ] 运营宽表列定义（ClickHouse）
- [ ] CI 与发布流程

## License

MIT