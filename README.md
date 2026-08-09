# go-observability

基于 [OpenTelemetry](https://opentelemetry.io/) 语义约定的 Go **语义化日志 + 三信号装配** 组件（源即规范）。

[![CI](https://github.com/formal-you/go-observability/actions/workflows/ci.yml/badge.svg)](https://github.com/formal-you/go-observability/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/formal-you/go-observability.svg)](https://pkg.go.dev/github.com/formal-you/go-observability)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> **模块路径**：`github.com/formal-you/go-observability`（公开发布前请确认 org/repo 与 `go.mod` 一致后再打 tag）。
>
> Agent/协作者硬规则：[AGENTS.md](AGENTS.md)

| | |
| --- | --- |
| **Go** | 1.25+（见 `go.mod`） |
| **文档语言** | 中文为主；术语保留英文（span / trace / semconv） |
| **版本** | v0.x 允许破坏性变更 → [CHANGELOG.md](CHANGELOG.md) |
| **许可证** | MIT |

## 文档地图

| 文档 | 说明 |
| --- | --- |
| [docs/README.md](docs/README.md) | **文档索引** |
| [docs/onboarding.md](docs/onboarding.md) | 15 分钟上手 |
| [docs/configuration.md](docs/configuration.md) | **配置总览**（env / Config / Logger / 部署） |
| [docs/environment.md](docs/environment.md) | 环境变量速查 |
| [docs/security.md](docs/security.md) | 脱敏与安全责任 |
| [docs/architecture.md](docs/architecture.md) | 架构与数据流 |
| [example/config/](example/config/) | **可复制配置（逐字段注释）** |
| [observability/](observability/) | 本地 LGTM 全栈 |
| [CONTRIBUTING.md](CONTRIBUTING.md) · [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) · [SECURITY.md](SECURITY.md) · [SUPPORT.md](SUPPORT.md) | 开源协作 |

## 特性

- 核心包零外部依赖（标准库）；属性键对齐 OTel semconv **1.41.0** + vendor **`app.*`**
- **三信号装配**（`internal/telemetry`）：Trace / Metric / Log + OTLP gRPC；B9 env 切换（disabled / JSONL / OTLP）
- **六类事件**：access / business / error / audit / security / probe；领域 `business.*` 由接入方自建（[`example/mall`](example/mall)）
- **errs**：Kind / Type v1 / AppError / StackRule；`LevelOf` + `EventFromError`（B2/B3）
- **Sampler / Masker**：`ResultKeepSampler`、`FieldMasker`（可选注入，C6）
- **Writer**：OTLP / stdout / file(JSONL)
- **中间件**：官方仅 Gin（`ginlog` + `recover`）；`net/http` 见 [`example/nethttp`](example/nethttp)
- **Metric**：库不内置业务指标；`Providers.Meter()` + [`example/metrics`](example/metrics)（B5）

## 快速开始

```bash
go get github.com/formal-you/go-observability@latest   # 公开仓库后
```

```go
import (
    "github.com/gin-gonic/gin"
    "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

    log "github.com/formal-you/go-observability"
    "github.com/formal-you/go-observability/middleware/ginlog"
    "github.com/formal-you/go-observability/writer/file"
)

w, _ := file.New("logs/events.jsonl")
logger := log.NewLogger(w,
    log.WithSampler(log.ResultKeepSampler{Ratio: 1}),
    log.WithMasker(log.FieldMasker{}),
)

r := gin.New()
r.Use(otelgin.Middleware("my-service"))
r.Use(ginlog.Middleware(ginlog.Config{Logger: logger}))
```

业务事件（领域名用接入方常量或 `NewEventName`）：

```go
logger.Emit(ctx, log.BusinessEvent{
    EventMetadata: log.EventMetadata{Level: log.LevelInfo, TraceID: tid, SpanID: sid},
    Data: log.BusinessPayload{
        EventName: log.NewEventName("business", "order", "paid"),
        Result:    log.ResultSuccess,
    },
})
```

### 配置怎么用

1. 复制 [example/config/.env.example](example/config/.env.example)（**每个变量都有类型/默认/行为注释**）
2. 应用结构对照 [example/config/app.example.yaml](example/config/app.example.yaml)
3. Collector：[example/config/collector.example.yaml](example/config/collector.example.yaml) 或 [observability/](observability/)
4. 说明文档：[docs/configuration.md](docs/configuration.md)

```bash
# 可选：本地 LGTM
docker compose -f observability/docker-compose.yml up -d

# 演示（未设 endpoint 时写 example/logs/events.jsonl）
go run ./example
```

更多 example 索引：[example/README.md](example/README.md)。

## 包布局

```
log/                      核心 API（types/keys/payload/events/normalize/logger/sampler/masker）
errs/                     错误体系（零外部依赖）
middleware/ginlog|recover Gin 集成（C3）
writer/{otlp,stdout,file} 后端
internal/telemetry        三信号装配
internal/attrkv           slog ↔ OTel
example/config            配置样例（字段注释）
example/{mall,metrics,nethttp,samber}
observability/            LGTM compose + templates
docs/                     用户文档
```

## OTel 符合性（摘要）

- 键名：semconv 1.41.0（`url.path`、`code.function.name` 等）+ `app.*`
- `event.name` → LogRecord EventName 顶层；双投影（JSONL 扁平 vs OTLP）
- `request_id`：显式优先，否则 trace_id 前缀 12 hex（A4）
- Resource：`service.name` / `version` / `deployment.environment` + region/instance

## 安全提示

- `FieldMasker` 默认只盖密钥类键，**不是**完整合规方案 → [docs/security.md](docs/security.md)
- 默认 trace 采样 **0.1** 为演示值，生产请改 `TraceSampleRatio`
- 漏洞报告 → [SECURITY.md](SECURITY.md)

## 验证

```bash
go test ./...
```

## Roadmap

- [x] OTLP / telemetry / 配置模板与 example/config 字段注释
- [x] ResultKeepSampler + FieldMasker
- [x] CI + 开源文档（CONTRIBUTING / CoC / SECURITY / SUPPORT）
- [ ] Grafana 面板精细化 + collector tail_sampling 判定属性
- [ ] 字段映射表导出 Collector transform
- [ ] 运营宽表（ClickHouse，接入方）

## License

MIT · [LICENSE](LICENSE)
