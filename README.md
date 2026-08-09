# 🔭 go-observability

<p align="center">
  <strong>让 Go 日志不再只是字符串</strong><br>
  用稳定事件模型，把 Log、Trace、Metric 接到同一条可观测链路。
</p>

<p align="center">
  <a href="https://github.com/formal-you/go-observability/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/formal-you/go-observability/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://go.dev/"><img alt="Go 1.25+" src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://opentelemetry.io/"><img alt="OpenTelemetry" src="https://img.shields.io/badge/OpenTelemetry-semconv%201.41.0-4f62ad"></a>
  <a href="LICENSE"><img alt="MIT License" src="https://img.shields.io/badge/License-MIT-blue.svg"></a>
</p>

<p align="center">
  <a href="#user-content-quick-start">🚀 快速开始</a> ·
  <a href="#user-content-event-model">🧩 事件模型</a> ·
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
| 🛡️ | **日志治理** | 可注入 Sampler、递归 Masker、写入错误回调 |
| 🚚 | **三种 Writer** | file、stdout、OTLP gRPC 共享同一套事件模型 |
| 📡 | **统一 telemetry** | Trace、Metric、Log Provider、Resource 与 Shutdown 集中装配 |
| 🧰 | **可运行参考栈** | Gin、net/http、Metric 示例与本地 LGTM 环境 |

---

## 👀 先看一眼结果

运行最小示例：

```bash
go run ./example/minimal
```

得到一条可直接检索的 JSONL 事件：

```json
{"app.result":"success","event.name":"business.order.paid","level":"INFO","msg":"business"}
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
        EventName: log.NewEventName("business", "order", "paid"),
        Result:    log.ResultSuccess,
    },
})

if err := w.Close(ctx); err != nil {
    return err
}
```

完整代码见 [`example/minimal/main.go`](example/minimal/main.go)。

> [!IMPORTANT]
> `Logger.Emit` 不返回 Writer 错误。生产接入必须配置 `WithErrorHandler`，并在退出前关闭 Writer 或 `telemetry.Providers`。

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
事件名三段式结构

access.http.request
   │      │      └── 动作 action
   │      └───────── 对象 object
   └──────────────── 领域 domain
```

框架级事件名由核心包维护；`business.*` 领域事件由接入方自建注册表。公共语义保持稳定，同时不会把电商、支付等领域概念硬塞进通用库。

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

⚙️ telemetry.SetupFromEnvironment
├── 🔍 TracerProvider
├── 📊 MeterProvider
├── 📝 LoggerProvider
└── 🧹 统一 Resource 与 Shutdown
```

更完整的数据流、所有权和关闭顺序见 [架构说明](docs/architecture.md)。

---

## 🧠 五个刻意做出的设计选择

### 🧱 1. 核心事件模型不绑定 OTel SDK

根包以标准库 `log/slog` 为属性载体。OTel 依赖集中在转换、Writer、telemetry 和集成层，业务代码不需要围绕 exporter 重写。

### 🪞 2. file/stdout 与 OTLP 不强行共用一种形状

本地 JSONL 需要 `level`、`event.name`、`trace_id` 等可直接检索的扁平列；OTLP 应使用 Severity、EventName、Timestamp 和 span context 顶层字段。双投影让两个目标都保持正确。

### 🧰 3. 日志治理默认显式注入

`NewLogger` 不会悄悄开启采样或脱敏。接入方必须明确选择 `ResultKeepSampler`、`FieldMasker` 和错误回调，避免“以为已经安全”的错误默认值。

### 🧨 4. 错误链是公共输入，不是实现细节

`EventFromError` 与 `LevelOf` 接受标准 `error`，沿 `%w` 链提取应用错误信息，并对普通 error 与 typed-nil 提供稳定兜底。

### 🎚️ 5. 头部采样和尾部采样不混淆

`TraceSampleRatio` 是 SDK 头部采样。Collector 无法恢复 SDK 已丢弃的 trace；需要按错误或延迟执行 `tail_sampling` 时，应先让 SDK 完整导出，再由 Collector 决策。

---

<a name="packages"></a>

## 📦 仓库里有什么

```text
go-observability/
├── 🧩 根包 log                  # Event、Payload、Logger、Sampler、Masker
├── 🧨 errs/                    # 错误分类、堆栈策略与错误投影
├── 🚚 writer/
│   ├── 📄 file/                # JSONL 文件 Writer
│   ├── 🖥️ stdout/              # 标准输出 Writer
│   └── 📡 otlp/                # OTLP gRPC Log Writer
├── ⚙️ telemetry/               # Trace、Metric、Log Provider 装配
├── 🔌 middleware/
│   ├── ginlog/                 # Gin access 日志
│   └── recover/                # Gin panic 恢复与错误事件
├── 🧪 example/                 # minimal、Gin、net/http、Metric、领域事件示例
├── 📊 observability/           # Collector、Loki、Tempo、Mimir、Grafana 参考栈
└── 📚 docs/                    # 接入、配置、架构、安全与发布文档
```

常用入口：[`example/`](example/) · [`telemetry/`](telemetry/) · [`observability/`](observability/) · [`docs/`](docs/)

---

## 📡 从本地 JSONL 切到 OTLP

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
