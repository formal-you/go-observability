# go-observability

> **让 Go 日志不再只是字符串：用稳定事件模型，把 Log、Trace、Metric 接到同一条可观测链路。**

<p align="center">
  <a href="https://github.com/formal-you/go-observability/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/formal-you/go-observability/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://go.dev/"><img alt="Go 1.25+" src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://opentelemetry.io/"><img alt="OpenTelemetry" src="https://img.shields.io/badge/OpenTelemetry-semconv%201.41.0-4f62ad"></a>
  <a href="LICENSE"><img alt="MIT License" src="https://img.shields.io/badge/License-MIT-blue.svg"></a>
</p>

开源仓库：https://github.com/formal-you/go-observability

---

你的服务是否也遇到过这些问题？

- 日志已经是 JSON，但每个模块仍在发明自己的字段，查一次问题要先猜键名。
- Trace、Metric、Log 分别初始化，资源属性、endpoint 和关闭顺序不断漂移。
- 错误经过 `%w` 包装后，retry、source、stack、upstream 信息悄悄丢失。
- 以为日志已经脱敏，嵌套 map、slice 或 `LogValuer` 里仍可能漏出 token。
- file、stdout、OTLP 共用一套“万能结构”，结果既不好读，也不符合 OTel LogRecord 语义。

**go-observability** 为这些问题提供一套开箱可用的语义层：事件类型稳定、字段来源明确、错误可投影、日志可治理、三信号可统一装配。它不试图替代 `slog` 或 OpenTelemetry，而是把两者之间最容易失控的约定固化成可测试的 Go API。

---

## 一分钟了解核心能力

| 能力 | 做了什么 | 直接收益 |
|:---|:---|:---|
| **六类类型化事件** | access、business、error、audit、security、probe | 团队不再用随意字符串表达不同语义 |
| **源即规范** | OTel semconv 1.41.0 + `app.*` vendor namespace | 字段名可追踪，避免每个服务各写一套 |
| **双投影** | JSONL/stdout 保留运营友好的扁平列；OTLP 映射 LogRecord 顶层字段 | 本地好查，后端也符合 OTel 语义 |
| **链路关联** | 从 context 关联 trace/span，OTLP 不重复写伪属性 | 日志和 Trace 可以沿同一请求定位 |
| **错误投影** | 支持普通 error、值/指针错误和 `%w` 链 | retry、source、stack、upstream 不再因包装丢失 |
| **日志治理** | 可注入 Sampler、递归 Masker、写入错误回调 | 控制噪音、敏感信息和静默丢日志风险 |
| **三种 Writer** | JSONL file、stdout、OTLP gRPC | 从本地开发到 Collector 无需换事件模型 |
| **公开 telemetry 包** | 统一装配 Trace、Metric、Log Provider 与 Resource | endpoint、采样、批量和 Shutdown 集中管理 |
| **框架与参考栈** | Gin 中间件、net/http 示例、本地 LGTM 配置 | 不只给接口，也给可运行的接入路径 |

---

## 先看一眼结果

从仓库根目录运行最小示例：

```bash
go run ./example/minimal
```

它会生成一条可以直接检索的 JSONL 事件：

```json
{"app.result":"success","event.name":"business.order.paid","level":"INFO","msg":"business"}
```

同一类事件切换到 OTLP Writer 后，severity、EventName、timestamp 和 span context 会进入 OTel LogRecord 顶层，业务属性继续保持结构化，不会退化成拼接字符串。

---

## 三分钟跑起来

### 方式一：克隆并运行示例

```bash
git clone https://github.com/formal-you/go-observability.git
cd go-observability
go run ./example/minimal
```

查看输出：

```powershell
Get-Content .\logs\events.jsonl
```

```bash
tail -n 1 ./logs/events.jsonl
```

### 方式二：预览期接入你的模块

项目尚未发布首个版本标签。现在可以从 `main` 获取预览版本：

```bash
go get github.com/formal-you/go-observability@main
```

最小埋点：

```go
ctx := context.Background()

w, err := file.New("logs/events.jsonl")
if err != nil {
    return err
}

logger := log.NewLogger(w,
    log.WithMasker(log.FieldMasker{}),
    log.WithErrorHandler(func(_ context.Context, event string, _ []slog.Attr, err error) {
        slog.Error("write observability event", "event", event, "err", err)
    }),
)

logger.Emit(ctx, log.BusinessEvent{
    EventMetadata: log.EventMetadata{Level: log.LevelInfo},
    Data: log.BusinessPayload{
        EventName: log.NewEventName("business", "order", "paid"),
        Result:    log.ResultSuccess,
    },
})

if err := w.Close(ctx); err != nil {
    return err
}
```

完整可运行代码见 [example/minimal/main.go](example/minimal/main.go)。`Logger.Emit` 不返回 Writer 错误，生产接入必须配置 `WithErrorHandler`，并在退出前关闭 Writer 或 `telemetry.Providers`。

---

## 六类事件，各自负责什么

| 事件 | 适用场景 | 典型例子 |
|:---|:---|:---|
| `AccessEvent` | 请求入口与响应结果 | HTTP method、route、status、duration |
| `BusinessEvent` | 领域动作与业务结果 | 下单成功、支付拒绝、库存预占 |
| `ErrorEvent` | 系统错误与依赖失败 | 超时、重试耗尽、panic、上游不可用 |
| `AuditEvent` | 可追责的数据或权限变更 | 配置修改、角色变更、订单状态变更 |
| `SecurityEvent` | 安全判断与阻断 | 登录失败、访问拒绝、风控命中 |
| `ProbeEvent` | 健康检查与运行状态 | readiness、liveness、依赖探测 |

框架级事件名由核心包维护；`business.*` 领域事件由接入方自建注册表。这样既保证公共语义稳定，也不会把电商、支付等领域概念硬塞进通用库。

---

## 数据怎么流动

```text
类型化 Event
    |
    v
Logger 统一元数据与字段归一化
    |
    +--> Sampler  控制低价值事件采样
    |
    +--> Masker   递归处理 group / map / slice / LogValuer
    |
    v
Writer
    +--> file     JSONL 扁平运营投影
    +--> stdout   本地与容器标准输出
    +--> OTLP     OTel LogRecord + context 中的 span 关联

telemetry.SetupFromEnvironment
    +--> TracerProvider
    +--> MeterProvider
    +--> LoggerProvider
    +--> 统一 Resource 与 Shutdown
```

更完整的数据流、所有权和关闭顺序见 [架构说明](docs/architecture.md)。

---

## 几个刻意做出的设计选择

### 1. 核心事件模型不绑定 OTel SDK

根包以标准库 `log/slog` 为属性载体。OTel 依赖集中在转换、Writer、telemetry 和集成层，业务代码不需要围绕某个 exporter 重写。

### 2. file/stdout 与 OTLP 不强行共用一种形状

本地 JSONL 需要 `level`、`event.name`、`trace_id` 等可直接检索的扁平列；OTLP 则应该使用 Severity、EventName、Timestamp 和 span context 顶层字段。双投影让两个目标都保持正确。

### 3. 日志治理默认显式注入

`NewLogger` 不会悄悄开启采样或脱敏。接入方需要明确选择 `ResultKeepSampler`、`FieldMasker` 和错误回调，避免“以为已经安全”的错误默认值。

### 4. 错误链是公共输入，不是实现细节

`EventFromError` 与 `LevelOf` 接受标准 `error`，沿 `%w` 链提取应用错误信息，并对普通 error 与 typed-nil 提供稳定兜底。

### 5. 头部采样和尾部采样不混淆

`TraceSampleRatio` 是 SDK 头部采样。Collector 无法恢复 SDK 已丢弃的 trace；需要按错误或延迟做 `tail_sampling` 时，应先让 SDK 完整导出，再由 Collector 决策。

---

## 包导航

| 包 | 用途 |
|:---|:---|
| 根包 `log` | Event、Payload、Logger、Sampler、Masker |
| [errs](errs/) | 错误分类、堆栈策略与错误投影 |
| [writer/file](writer/file/) | JSONL 文件 Writer |
| [writer/stdout](writer/stdout/) | 标准输出 Writer |
| [writer/otlp](writer/otlp/) | OTLP gRPC Log Writer |
| [telemetry](telemetry/) | Trace、Metric、Log Provider 装配 |
| [middleware/ginlog](middleware/ginlog/) | Gin access 日志 |
| [middleware/recover](middleware/recover/) | Gin panic 恢复与错误事件 |
| [example](example/) | 最小、Gin、net/http、Metric、领域事件示例 |
| [observability](observability/) | Collector、Loki、Tempo、Mimir、Grafana 参考栈 |

---

## 从本地 JSONL 切到 OTLP

主示例通过环境变量选择出口：

```powershell
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example
```

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example
```

裸 `host:port` 明确按明文 OTLP gRPC 处理；`http://` 与 `https://` URL 保留各自语义。非法 host、端口、path、query 或 fragment 会直接返回错误，不会静默回退到默认地址。

需要完整本地后端时，使用 [observability](observability/) 中的参考栈：

```bash
docker compose -f observability/docker-compose.yml up -d
```

配置字段和生产边界见 [配置指南](docs/configuration.md) 与 [环境变量速查](docs/environment.md)。

---

## 安全不是一句“已脱敏”

- `FieldMasker` 会递归处理 group、map、slice 和 `LogValuer`，默认覆盖 password、token、authorization 等常见凭证键。
- 默认键清单不是完整 PII 合规方案；手机号、证件号、业务标识等必须由接入方维护。
- `BusinessPayload.ExtraAttrs` 不能覆盖身份、资源、业务结果、代码位置等 canonical 字段。
- 审计与安全日志的防篡改存储、访问控制、保留期限仍由使用方负责。

完整责任边界见 [安全指南](docs/security.md)。漏洞请通过 [GitHub 私密报告入口](https://github.com/formal-you/go-observability/security/advisories/new) 提交，不要公开利用细节。

---

## 质量门禁

每次 push 和 PR 都会在 GitHub Actions 中执行：

| 平台 | 检查 |
|:---|:---|
| Ubuntu | module verify、gofmt、vet、test、race、govulncheck |
| Windows | module verify、vet、test |

本地验证：

```bash
go build ./...
go vet ./...
go test ./...
```

`main` 已启用分支保护：必须走 PR、通过双平台检查、保持线性历史，禁止强推与删除。

---

## 当前发布状态

仓库处于公开预览阶段，公共 API 仍可能在首个 tag 前调整。首次正式发布将完成：

- 整理 `CHANGELOG.md` 的 `[Unreleased]` 内容；
- 创建并推送 `v0.1.0`；
- 在全新模块中验证带版本号的 `go get`；
- 检查 pkg.go.dev 索引和外部文档链接。

进度见 [发布检查清单](docs/release-checklist.md) 和 [CHANGELOG](CHANGELOG.md)。

---

## 文档与参与贡献

| 文档 | 内容 |
|:---|:---|
| [文档索引](docs/README.md) | 全部用户文档入口 |
| [快速接入](docs/onboarding.md) | 从零接入 Logger 与 telemetry |
| [配置指南](docs/configuration.md) | Config、环境变量与 Writer 选择 |
| [架构说明](docs/architecture.md) | 分层、数据流与所有权 |
| [贡献指南](CONTRIBUTING.md) | 开发、验证与 PR 要求 |
| [安全政策](SECURITY.md) | 私密漏洞报告与处理原则 |
| [支持渠道](SUPPORT.md) | Issue 分类与求助边界 |

欢迎提交 Issue 和 PR。领域事件、框架适配、文档案例和互操作验证都很有价值，但请保持核心包的通用边界，并为行为变化补充测试。

---

## License

[MIT License](LICENSE)，可自由使用、修改和分发。

---

<p align="center">
  <strong>让每一条事件都有稳定语义，让每一次故障都有可追踪上下文。</strong><br/>
  <em>go-observability · semantic events for Go services</em>
</p>
