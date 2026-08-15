# 🔭 go-observability

<p align="center">
  <strong>让 Go 日志不再只是字符串</strong><br>
  用稳定事件模型，把 Log、Trace、Metric 接到同一条可观测链路。
</p>

<p align="center">
  <a href="https://github.com/formal-you/go-observability/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/formal-you/go-observability/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://go.dev/"><img alt="Go 1.26+" src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://opentelemetry.io/"><img alt="OpenTelemetry" src="https://img.shields.io/badge/OpenTelemetry-semconv%201.41.0-4f62ad"></a>
  <a href="LICENSE"><img alt="MIT License" src="https://img.shields.io/badge/License-MIT-blue.svg"></a>
</p>

<p align="center">
  <a href="#user-content-exports">🚦 出口组合</a> ·
  <a href="#user-content-quick-start">🚀 快速开始</a> ·
  <a href="#user-content-event-model">🧩 事件模型</a> ·
  <a href="#user-content-registry">🗂️ 语义注册表</a> ·
  <a href="#user-content-data-flow">🗺️ 数据流</a> ·
  <a href="#user-content-packages">📦 包导航</a> ·
  <a href="#user-content-security">🛡️ 安全边界</a>
</p>

---

## 🎯 你的日志，可能“有格式”但没有语义

```text
😵 字段漂移        每个模块都在发明自己的 key，查问题先猜字段名
🔗 链路断裂        Log、Trace、Metric 各自初始化，故障现场拼不起来
🧨 错误失真        error 经 %w 包装后，retry、source、stack 悄悄丢失
🔓 脱敏有漏网      token 藏在 map、slice、group 或 LogValuer 里继续外泄
📦 出口形状混乱    file、stdout、OTLP 共用“万能结构”，两边都不好用
```

> [!TIP]
> **go-observability 是 `slog` 与 OpenTelemetry 之间的语义层。** 事件类型稳定、字段来源明确、错误可投影、日志可治理，三信号可以统一装配。它不替代 `slog` 或 OTel，而是把团队最容易失控的约定固化成可测试的 Go API。

---

## ✨ 一眼看懂它能做什么

| | 能力 | 你直接得到什么 |
|:---:|:---|:---|
| 🧩 | **六类类型化事件** | access、business、error、audit、security、probe 不再靠随意字符串区分 |
| 📐 | **源即规范** | OTel semconv 1.41.0 + `app.*` vendor namespace，字段名可追踪 |
| 🪞 | **双投影** | JSONL/stdout 保留运营友好扁平列，OTLP 映射正确的 LogRecord 顶层字段 |
| 🔗 | **链路关联** | 从 context 关联 trace/span，不重复制造伪属性 |
| 🧨 | **错误投影** | 普通 error、值/指针错误和 `%w` 链都能稳定提取 |
| 🗂️ | **语义注册表** | error.code、error.type、event.name 的固定映射与校验，漂移早暴露 |
| 🛡️ | **日志治理** | 可注入 Sampler、递归 Masker、写入错误回调 |
| 🚚 | **三种 Writer** | file、stdout、OTLP gRPC 共享同一套事件模型 |
| 🛡️ | **Security / Audit 中间件** | `SecurityLog` / `AuditLog`（gin + net/http）把认证/授权判定与审计留痕自动写成事件 |
| 📡 | **统一 telemetry** | Trace、Metric、Log Provider、Resource 与 Shutdown 集中装配 |
| 🧰 | **可运行参考栈** | Gin、net/http、Metric 示例与本地 LGTM 环境 |

<a name="exports"></a>

## 🚦 出口组合速查

`Enabled` 是总开关；`Trace` / `Metric` / `Log` 各自选择出口，常见组合如下：

| 场景 | Enabled | Trace | Metric | Log | 说明 |
| --- | --- | --- | --- | --- | --- |
| 生产全链路 | true | otlp | otlp | otlp | 三信号发 Collector，需 `Endpoint` |
| 本地开发 | true | stdout | stdout | stdout | 三信号全打 stdout |
| 混合出口 | true | otlp | file | file | Trace 进 Collector，Metric/Log 落本地 |
| file-only 小单体 | true | local | none | file | 用 `NewFileRuntime` |
| 只写本地日志 | false | 忽略 | 忽略 | file/stdout/none | 不创建 Provider，只保留 Log Writer |
| 完全关闭 | false | 忽略 | 忽略 | none | 不采集、不写日志 |

三个常见档位已有快捷函数：

```go
// 只开日志
runtime, _ := telemetry.NewLogRuntime(ctx, "order-api", telemetry.SignalOutputFile, "logs/events.jsonl")

// 全 OTLP
runtime, _ = telemetry.NewOTLPRuntime(ctx, "order-api", "otel-collector:4317")

// 全文件
runtime, _ = telemetry.NewAllFileRuntime(ctx, "order-api", "logs")
```

完整字段与配置模板见 [配置指南](docs/configuration.md)。

---

## 👀 先看一眼结果

运行最小示例：

```bash
go run ./example/minimal
```

得到一条可直接检索的 JSONL 事件：

```json
{"timestamp":"2026-08-11T15:00:00Z","level":"INFO","type":"business","service.name":"mall-monolith","deployment.environment.name":"development","trace_id":"...","span_id":"...","event.name":"order.payment.succeeded","app.result":"success"}
```

切换到 OTLP Writer 后，`severity`、`EventName`、`timestamp` 和 span context 会进入 OTel LogRecord 顶层；业务属性继续保持结构化，不会退化成拼接字符串。

> [!NOTE]
> 同一个 Event，可以面向本地检索和 OTel 后端生成不同但都正确的投影。业务代码不需要因为出口变化而重写埋点。

---

<a name="quick-start"></a>

## 🚀 三分钟跑起来

### 1️⃣ 克隆并运行

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

### 2️⃣ 接入你的 Go 模块

项目尚未发布首个版本标签，预览期从 `main` 获取：

```bash
go get github.com/formal-you/go-observability@main
```

### 3️⃣ 发出第一条类型化事件

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
        EventName: log.NewEventName("order", "payment", "succeeded"),
        Result:    log.ResultSuccess,
    },
})

if err := w.Close(ctx); err != nil {
    return err
}
```

完整代码见 [`example/minimal/main.go`](example/minimal/main.go)。

> [!IMPORTANT]
> `Logger.Emit` 不返回 Writer 错误。生产接入必须配置 `WithErrorHandler`，并在退出前关闭 Writer，随后调用 `telemetry.Runtime.Shutdown`。

---

<a name="event-model"></a>

## 🧩 六类事件，各司其职

| 图标 | 事件 | 适用场景 | 典型例子 |
|:---:|:---|:---|:---|
| 🌐 | `AccessEvent` | 请求入口与响应结果 | method、route、status、duration |
| 🛒 | `BusinessEvent` | 领域动作与业务结果 | 下单成功、支付拒绝、库存预占 |
| 🚨 | `ErrorEvent` | 系统错误与依赖失败 | 超时、重试耗尽、panic、上游不可用 |
| 🧾 | `AuditEvent` | 可追责的数据或权限变更 | 配置修改、角色变更、订单状态变更 |
| 🛡️ | `SecurityEvent` | 安全判断与阻断 | 登录失败、访问拒绝、风控命中 |
| 🩺 | `ProbeEvent` | 健康检查与运行状态 | readiness、liveness、依赖探测 |

```text
事件名结构：event.name MUST use the form <domain>.<subject>.<event>（`type` 已承载粗分类）

http.request.completed
   │      │      └── <event> 注册的 Event Type
   │      └───────── <subject> 对象
   └──────────────── <domain> 领域
```

`event.name` MUST use the form `<domain>.<subject>.<event>`（正则 `EventNamePattern` 校验）；`<event>` MUST 是注册的 Event Type（稳定语义发生，唯一标识 Event Structure），不是自由文本，也不是 Operation Lifecycle Stage（生命周期经 Span 建模）；首段不得是 `access` / `business` / `error` / `security` / `audit` / `probe`，这些粗分类只由 `type` 表达。框架级事件名由核心包维护，领域事件由接入方自建注册表。

<a name="registry"></a>

## 🗂️ 语义注册表：error.code、error.type、event.name 不再自由漂移

日志系统最容易退化的地方不是“怎么写”，而是“怎么命名”。这里的三个注册表把最容易漂移的三个字段变成可枚举、可校验、冲突早失败的 Go API：

| 注册表 | 管什么 | 关键入口 |
|:---|:---|:---|
| `EventName` | `event.name` 必须是 `<domain>.<subject>.<event>`，首段不能重复六类 `type` | `log.EventNamePattern`、`log.NewEventName`、框架级常量 / 接入方领域常量 |
| `ErrorType` | `error.type` 必须是 16 个 OTel/gRPC canonical code 的闭合枚举 | `errs.ErrorType.Validate`、`errs.ParseErrorType` |
| `Error Registry` | `error.code → error.type`，可选再绑定一个错误 `event.name` | `RegisterErrorCode` / `RegisterErrorContract` / `MustRegister...` / `RegisteredErrorType` / `RegisteredEventName` |

框架级事件名登记在 `log/types.go`；接入方领域事件名以常量维护自己的注册表（见 [`example/mall`](example/mall/README.md)），禁止在业务代码里散落手写字符串。

错误注册表在启动期一次性写入，并在严格构造器中校验：同一个 `error.code` 只能映射到唯一的 `error.type`（以及可选的唯一错误事件名）。

```go
func init() {
    // 业务/校验错误：只注册 code → type；领域事件由 Application 另行发布
    errs.MustRegisterErrorCode("ORDER.CREATE.STOCK_INSUFFICIENT", errs.TypeFailedPrecondition)

    // 系统/基础设施错误：注册 code → type + 错误事件名
    // errs 不依赖 log，所以事件名文法先由接入方按 log.EventNamePattern 校验
    if err := log.EventName("db.query.deadline_exceeded").Validate(); err != nil {
        panic(err)
    }
    errs.MustRegisterErrorContract(
        "INFRA.MYSQL.QUERY_TIMEOUT",
        errs.TypeDeadlineExceeded,
        "db.query.deadline_exceeded",
    )
}
```

注册表可以反查，严格构造器会拒绝漂移：

```go
typ, ok := errs.ErrorCode("INFRA.MYSQL.QUERY_TIMEOUT").RegisteredErrorType()
name, hasName := errs.ErrorCode("INFRA.MYSQL.QUERY_TIMEOUT").RegisteredEventName()

// 注册表要求 DEADLINE_EXCEEDED，下面的构造会在启动期直接失败
_, err := errs.NewSystemError(errs.SystemErrorConfig{
    Type:    errs.TypeUnavailable,
    Code:    "INFRA.MYSQL.QUERY_TIMEOUT",
    Message: "database timeout",
})
// err != nil：error.code 已注册为 DEADLINE_EXCEEDED
_ = typ
_ = ok
_ = name
_ = hasName
```

---

<a name="data-flow"></a>

## 🗺️ 一条事件如何走完全程

```text
🧩 类型化 Event
│
└──▶ 🧠 Logger：统一元数据与字段归一化
     │
     ├──▶ 🎚️ Sampler：控制低价值事件采样
     ├──▶ 🛡️ Masker：递归处理 group / map / slice / LogValuer
     │
     └──▶ 🚚 Writer
          ├── 📄 file     → JSONL 扁平运营投影
          ├── 🖥️ stdout   → 本地与容器标准输出
          └── 📡 OTLP     → OTel LogRecord + span context

⚙️ telemetry.NewRuntime + InstallGlobal
├── 🔍 TracerProvider
├── 📊 MeterProvider
├── 📝 LoggerProvider
└── 🧹 统一 Resource 与 Shutdown
```

更完整的数据流、所有权和关闭顺序见 [架构说明](docs/architecture.md)。

---

## 🧠 五个刻意做出的设计选择

### 🧱 1. 核心事件模型不绑定 OTel SDK

log 包以标准库 `log/slog` 为属性载体。OTel 依赖集中在转换、Writer、telemetry 和集成层，业务代码不需要围绕 exporter 重写。

### 🪞 2. file/stdout 与 OTLP 不强行共用一种形状

本地 JSONL 需要 `level`、`event.name`、`trace_id` 等可直接检索的扁平列；OTLP 应使用 Severity、EventName、Timestamp 和 span context 顶层字段。双投影让两个目标都保持正确。

### 🧰 3. 日志治理默认显式注入

`NewLogger` 不会悄悄开启采样或脱敏。接入方必须明确选择 `ResultKeepSampler`、`FieldMasker` 和错误回调，避免“以为已经安全”的错误默认值。

认证边界可以把可信身份放入 `log.WithIdentityContext`，并在 Logger 上配置
`log.WithIdentityExtractor(log.ContextIdentityExtractor{})`。`Subject.UserID` 新事件输出为
`user.id`，租户为 `app.tenant_id`；Security/Audit 的 Actor 输出为 `app.actor_user_id` /
`app.actor_role`，可信非空字段会覆盖伪造的事件属性。`FieldMasker{}` 内置处理凭证、Cookie、
手机号、证件号和常见 request body 键，生产代码仍需显式启用并补充组织特有敏感键。

系统错误的堆栈通过 `errs.SetStackConfig(errs.ProductionStackConfig())` 做大小和路径治理；
超限会输出 `app.stacktrace_truncated=true`，panic 不允许关闭诊断堆栈。

### 🧨 4. 错误链是公共输入，不是实现细节

`EventFromError` 与 `LevelOf` 接受标准 `error`，沿 `%w` 链提取应用错误信息，并对普通 error 与 typed-nil 提供稳定兜底。

### 🎚️ 5. 头部采样和尾部采样不混淆

`TraceSampleRatio` 是 SDK 头部采样。Collector 无法恢复 SDK 已丢弃的 trace；需要按错误或延迟执行 `tail_sampling` 时，应先让 SDK 完整导出，再由 Collector 决策。

## 🎚️ 默认全量与可选采样

日志条数由应用侧控制。`NewLogger` 未配置 `WithSampler` 时每条都写，这是推荐默认：

- **HTTP 访问事件（access.\*）**：除明确跳过的健康检查外，每个请求恰好保留一条，无论结果是 2xx、4xx、5xx 还是 panic。
- **HTTP 事件关联**：HTTP 来源的 business（包括 success / failed）、error、security、audit 事件必须能通过同一 `trace_id` / `request_id` 找到该请求的 AccessEvent；一个请求中的多个业务事件仍只对应一条 AccessEvent。
- **后台事件边界**：MQ 消费、定时任务等非 HTTP 来源事件独立记录，不伪造 AccessEvent。
- **业务 / 错误 / 安全 / 审计 / 探测事件**：事件一旦产生就全量保留；业务成功同样是有效运营记录。
- **心跳 / 健康检查**：确定性噪音，用 `SkipPaths`（ginlog）或接入层短路直接排除，不进入概率采样。

AccessEvent 记录 method、path、status、latency 等请求事实，BusinessEvent 记录领域动作与结果，两者不是重复日志。即使业务结果为 success，运营仍需要 AccessEvent 计算请求量、成功率、延迟和容量，并在 trace 被采样时保留可检索的请求全貌。

高流量服务只有在已有网关全量 access，或明确接受成功请求的事件关联不完整时，才应显式启用 access 采样；一旦启用，就不再保证每个 HTTP 语义事件都有 AccessEvent。失败结果仍由 `ResultKeepSampler` 高优先级保留（Sampling/Retention Policy 层 SHOULD）：

```go
log.WithSampler(log.NewEventTypeKeepSampler(
	[]log.EventType{log.EventBusiness, log.EventError, log.EventSecurity, log.EventAudit, log.EventProbe},
	log.ResultKeepSampler{Ratio: 0.1},
))
```

`EventKeepSampler` 仍保留用于按领域前缀采样；旧的类别前缀配置会按 `type` 兼容识别。启用任意 AccessEvent 采样策略后，不再保证每个 HTTP 语义事件都有 AccessEvent。

---

<a name="packages"></a>

## 📦 仓库里有什么

```text
go-observability/
├── 🧩 log/ 子包                # Event、Payload、Logger、Sampler、Masker
├── 🧨 errs/                    # 错误分类、堆栈策略与错误投影
├── 🚚 writer/
│   ├── 📄 file/                # JSONL 文件 Writer
│   ├── 🖥️ stdout/              # 标准输出 Writer
│   └── 📡 otlp/                # OTLP gRPC Log Writer
├── ⚙️ telemetry/               # Trace、Metric、Log Provider 装配
├── 🔌 middleware/               # 按框架体系分组（gin / http / grpc / kratos）
│   ├── gin/                    # Gin：access / error / recover / security / audit / trace / metrics
│   ├── http/                   # net/http：error / recover / security / audit / trace / metrics
│   ├── grpc/                   # gRPC：trace / metrics 拦截器
│   └── kratos/                 # kratos v3 传输适配
├── 🧪 example/                 # minimal、Gin、net/http、Metric、领域事件示例
├── 📊 observability/           # Collector、Loki、Tempo、Mimir、Grafana 参考栈
└── 📚 docs/                    # 接入、配置、架构、安全与发布文档
```

常用入口：[`example/`](example/) · [`telemetry/`](telemetry/) · [`observability/`](observability/) · [`docs/`](docs/)

---

## 📡 从本地 JSONL 切到 OTLP

不部署 Collector 的小单体可直接使用 `telemetry.NewFileRuntime`：它不会创建 OTLP
exporter，只生成本地有效 trace/span 用于日志关联，并把 `service.name`、
`service.version`、`service.instance.id`、`deployment.environment.name` 写入每条 JSONL。
完整 Trace 树不会落盘；需要 Tempo 查询时再切换到 OTLP 模式。配置模板见
[`example/config/file-only.example.yaml`](example/config/file-only.example.yaml)。
带最低级别、输出路径和文件轮转的可运行配置见
[`example/blackbox/config.example.yaml`](example/blackbox/config.example.yaml)。

需要 Collector 时，使用 `telemetry.NewRuntime`，在 `Trace` / `Metric` / `Log` 分组中显式选择 `SignalOutputOTLP`；本地文件、容器标准输出和禁用日志分别使用 `SignalOutputFile`、`SignalOutputStdout` 和 `SignalOutputNone`。日志 Writer 参数进入 `Config.Log`，通过 `Runtime.NewWriter(ctx)` 创建。Runtime 构造不会修改 OTel 全局状态，应用需显式调用 `InstallGlobal` 并在退出时调用恢复函数。

主示例通过环境变量选择出口：

```powershell
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example
```

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example
```

```text
127.0.0.1:4317         → 明文 OTLP gRPC
http://collector:4317  → 明文 URL 语义
https://collector:4317 → TLS URL 语义
```

非法 host、端口、path、query 或 fragment 会直接返回错误，不会静默回退到默认地址。

启动完整本地后端：

```bash
docker compose -f observability/docker-compose.yml up -d
```

配置边界见 [配置指南](docs/configuration.md) 与 [环境变量速查](docs/environment.md)。

---

<a name="security"></a>

## 🛡️ 安全不是一句“已脱敏”

| ✅ 已提供 | ⚠️ 仍由接入方负责 |
|:---|:---|
| 递归处理 group、map、slice、`LogValuer` | 维护手机号、证件号等业务 PII 键清单 |
| 默认覆盖 password、token、authorization 等凭证键 | 配置审计日志的防篡改存储与访问控制 |
| 禁止 `ExtraAttrs` 覆盖 canonical 字段 | 制定日志保留期限与合规策略 |
| 支持显式写入错误回调 | 监控 Writer 失败并建立告警 |
| `InputGuard` 注入点：错误事件后按应用规则补发 `SecurityEvent` / `AuditEvent` | 风险分级/命中规则与输入摘要提取（`httperr.WithInputSummary`） |

完整责任边界见 [安全指南](docs/security.md)。漏洞请通过 [GitHub 私密报告入口](https://github.com/formal-you/go-observability/security/advisories/new) 提交，不要公开利用细节。

---

## ✅ 每次提交都过哪些门禁

```text
🐧 Ubuntu  → module verify → gofmt → vet → test → race → govulncheck
🪟 Windows → module verify → vet → test
```

本地验证：

```bash
go build ./...
go vet ./...
go test ./...
```

`main` 已启用分支保护：必须走 PR、通过双平台检查、保持线性历史，禁止强推与删除。

---

## 🏷️ 当前发布状态

> [!WARNING]
> 仓库处于 **公开预览阶段**，公共 API 在首个 tag 前仍可能调整。生产接入请固定 commit，并先完成兼容性验证。

首次正式发布将完成：

- 📝 整理 `CHANGELOG.md` 的 `[Unreleased]` 内容；
- 🏷️ 创建并推送 `v0.1.0`；
- 📦 在全新模块中验证带版本号的 `go get`；
- 🔎 检查 pkg.go.dev 索引和外部文档链接。

进度见 [发布检查清单](docs/release-checklist.md) 和 [CHANGELOG](CHANGELOG.md)。

---

## 📚 文档入口

| | 文档 | 适合什么时候看 |
|:---:|:---|:---|
| 🚀 | [快速接入](docs/onboarding.md) | 第一次接入 Logger 与 telemetry |
| ⚙️ | [配置指南](docs/configuration.md) | 选择 Config、环境变量与 Writer |
| 🏗️ | [架构说明](docs/architecture.md) | 理解分层、数据流与所有权 |
| 🛡️ | [安全政策](SECURITY.md) | 查看漏洞报告与处理原则 |
| 🤝 | [贡献指南](CONTRIBUTING.md) | 准备开发、验证和提交 PR |
| 🧭 | [文档索引](docs/README.md) | 浏览全部用户文档 |
| 💬 | [支持渠道](SUPPORT.md) | 提交 Issue 或定位求助入口 |

---

## 🤝 参与贡献

欢迎提交 Issue 和 PR。领域事件、框架适配、文档案例和互操作验证都很有价值，但请保持核心包的通用边界，并为行为变化补充测试。

```text
发现问题 🐛  → 提交 Issue → 对齐行为边界 → 补测试与实现 → PR + CI ✅
```

---

## 📄 License

[MIT License](LICENSE)，可自由使用、修改和分发。

---

<p align="center">
  <strong>🔭 让每一条事件都有稳定语义，让每一次故障都有可追踪上下文。</strong><br>
  <sub>go-observability · semantic events for Go services</sub>
</p>
