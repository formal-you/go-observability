# 架构说明

> 本文档与 `README.md`、`docs/configuration.md`、`AGENTS.md` 共同构成仓库内"项目真源"。
> 用途：作为**代码 Review 的导航图**与改动前的边界核对清单；文档与代码不一致时以代码为准，并在同一提交中修正文档。

## 1. 项目定位与设计原则

一句话定位：**go-observability 是 `slog` 与 OpenTelemetry 之间的语义层**——事件类型稳定、字段来源明确、错误可投影、日志可治理，三信号统一装配。它不替代 `slog` 或 OTel，而是把团队最容易失控的约定固化成可测试的 Go API。

- **方案2「源即规范」**：属性键直接使用 OTel semconv 1.41.0 名 + `app.*` vendor 命名空间，字段名可追踪、不发明私有键。
- **核心零依赖**：`log/` 包只依赖标准库（`log/slog`、`net`、`time`、`fmt`、`strings`）；OTel SDK 依赖只允许出现在 `internal/attrkv`、`writer/*`、`telemetry`、`middleware/*`、`example/*`。
- **批量导出分两层**：SDK 侧由 `telemetry.Config` 的批量导出间隔控制（trace 5s / metric 15s / log 1s），Collector 侧由 batch processor（timeout / send_batch_size）二次凑批；核心层每次 `Emit` 同步写出，不做批处理/定时器。
- **出口可替换**：同一事件模型可投影到 JSONL / stdout / OTLP，业务埋点不因出口变化而重写。
- **决策记录（ADR）**：错误模型（ErrorType / ErrorCode）等关键决策记录在 [docs/adr/](adr/README.md)，Review 时对照 ADR 判断实现是否漂移。

## 2. 分层架构

| 层 | 包 | 职责 |
| --- | --- | --- |
| 事件核心 | `log/` | 事件类型、属性键、字段归一化、Logger、采样与脱敏接口 |
| 错误模型 | `errs` | 错误分类（ErrorKind）、低基数失败类别（ErrorType）、堆栈策略（StackRule） |
| 错误投影 | `log/error_project.go` | 把 `errs.AppError` 沿错误链投影为 Business/Error 事件 |
| 写出 | `writer/file`、`writer/stdout`、`writer/otlp` | 把归一化事件写到不同后端（装配层，可替换） |
| OTel 映射 | `internal/attrkv` | `slog.Attr` ↔ OTel 值转换 + LogRecord 顶层字段映射（唯一核心映射层） |
| endpoint 校验 | `internal/otlpendpoint` | OTLP gRPC endpoint 统一校验与规范化（host:port / http(s) URL） |
| 三信号装配 | `telemetry` | 创建并关闭 Trace / Metric / Log Provider，选择日志出口 |
| HTTP/gRPC 集成 | `middleware/httperr`（契约核心）、`middleware/otelutil`（OTel 工具）、`middleware/gin`、`middleware/http`、`middleware/grpc`、`middleware/kratos` | 按框架体系分组：gin（Gin）、http（net/http）、grpc（gRPC）各含错误收口/access/链路/指标；kratos v3 见 `middleware/kratos`（HTTP ErrorEncoder + gRPC ErrorMapper + 错误日志 filter） |

依赖方向：`errs` 与 `log/` 互不依赖对方实现（log 包经 `EventFromError` 消费 `errs.AppError` 接口，`errs` 不依赖 log 包）；`middleware` 依赖 log 包与 `errs`（ginlog 只依赖 log 包，errresp/recover 还依赖 `errs`）；`writer/*`、`telemetry` 依赖 log 包与 `internal/*`。

## 3. 文件树（代码导航图）

```text
go-observability/
├── .github/
│   ├── ISSUE_TEMPLATE/          # bug / feature 报告模板
│   ├── dependabot.yml           # 依赖更新
│   ├── pull_request_template.md
│   └── workflows/ci.yml         # 双平台门禁：Ubuntu(verify/gofmt/vet/test/race/vuln) + Windows(vet/test)
│
├── log/                        # 核心日志包（零 OTel 依赖，只依赖标准库）
│   ├── doc.go                   # 包注释：核心包零外部依赖承诺
│   ├── types.go                 # 类型化枚举：EventType / EventName(三段式+Validate) / Level / Result / EventPayload
│   ├── keys.go                  # 属性键常量：semconv 1.41.0 + app.* vendor 命名空间
│   ├── metadata.go              # EventMetadata + Subject / Actor / Resource / Source / HTTPInfo
│   ├── payload.go               # 六类载荷：Access / Business / Error / Audit / Security / Probe
│   ├── events.go                # 六类事件结构体（EventMetadata + Data）
│   ├── normalize.go             # 归一化：metadata+payload → 扁平 attrs；reservedKeys 过滤；零值省略
│   ├── log.go                   # Logger / Writer / Sampler / Masker / ErrorHandler / Option
│   ├── sampler.go               # ResultKeepSampler（高价值结果强制保留）+ EventKeepSampler（事件前缀全量）
│   ├── sampler_rand.go          # 并发安全随机源（math/rand/v2）
│   ├── masker.go                # FieldMasker：按键名递归脱敏（group/map/slice/LogValuer）
│   ├── error_project.go         # EventFromError / LevelOf：errs.AppError → 事件投影
│   ├── error_project_test.go    # 投影黑盒测试（值/指针/错误链/nil）
│   ├── event_test.go            # 事件结构体与归一化测试
│   ├── logger_test.go           # Logger 管线测试
│   ├── sampler_masker_test.go   # 采样/脱敏测试
│   └── blackbox_log_test.go     # 外部包黑盒测试（验证对外契约）
│
├── errs/
│   ├── errs.go                  # ErrorKind / ErrorType / ErrorCode / AppError / BizError / SystemError / StackRule
│   └── errs_test.go
│
├── internal/                    # 内部共享（不公开 API）
│   ├── attrkv/
│   │   ├── attrkv.go            # slog.Attr ↔ OTel KeyValue；Record() 组装 LogRecord 顶层字段
│   │   └── attrkv_test.go
│   └── otlpendpoint/
│       ├── endpoint.go          # OTLP gRPC endpoint 校验与规范化
│       └── endpoint_test.go
│
├── writer/                      # 写出后端（实现 log 包 Writer，可替换）
│   ├── file/file.go             # JSONL 文件 Writer（append，并发安全，含 Close）
│   ├── stdout/stdout.go         # stdoutlog exporter 包装（本地演示）
│   └── otlp/otlp.go             # OTLP gRPC Writer（BatchProcessor；可注入外部 LoggerProvider）
│
├── middleware/               # 按框架体系分组（gin / http / grpc / kratos），共享契约与工具独立成包
│   ├── httperr/httperr.go       # 框架无关错误契约核心：Kind→状态码、安全 reason/message/metadata、扁平响应体、span 元数据
│   ├── otelutil/otelutil.go     # 框架无关 OTel 工具：链路注入/提取、TraceExtractor
│   ├── gin/                     # Gin 体系：AccessLog / ErrorResponse / Recover / SecurityLog / AuditLog / Trace / Metrics / Abort
│   ├── http/                    # net/http 体系：ErrorResponse / Recover / SetError / SetSecurity / SetAudit / SecurityLog / AuditLog / Trace / Metrics
│   ├── grpc/                    # gRPC 体系：Trace / Metrics unary 拦截器
│   ├── kratos/                  # kratos v3 适配：HTTP ErrorEncoder + gRPC ErrorMapper + 错误日志 filter（httperr 契约 + kratos 原生错误双识别）
│   └── internal/mwutil/         # 内部共享底层工具（状态码记录、路由/RPC 解析、span 收尾、直方图）
│
├── telemetry/
│   └── telemetry.go             # 三信号 Provider 装配 + Shutdown + NewLogWriter 出口选择
│
├── observability/               # 本地 LGTM 参考栈（docker compose，非生产方案）
│   ├── docker-compose.yml       # Collector + Tempo + Loki + Mimir + Grafana
│   ├── otel-collector-config.yaml / loki.yaml / mimir.yaml / tempo.yaml
│   ├── grafana/provisioning/    # 预置 datasource + overview dashboard
│   └── templates/               # 分信号管线模板 + 告警/指标命名示例
│
├── example/                     # 接入方示范（samber 生态只允许出现在这里）
│   ├── README.md                # 示例索引
│   ├── main.go                  # 主示例：Gin + telemetry + OTLP/JSONL 出口选择
│   ├── config/                  # 应用 / Collector 配置模板（.env.example、docker-compose.example）
│   ├── minimal/main.go          # 不依赖框架的最小 JSONL 示例
│   ├── mall/                    # 领域事件注册表示范：business.* 事件名 + app.* 专属键
│   ├── metrics/main.go          # 使用方定义 RED 与业务指标
│   ├── nethttp/main.go          # 标准库 HTTP access 事件
│   └── samber/main.go           # 与 samber slog handler 互操作
│
├── docs/                        # 用户文档（文档索引见 docs/README.md）
│   ├── architecture.md          # 本文档
│   ├── configuration.md / environment.md / onboarding.md / workflow.md
│   ├── security.md / samber-comparison.md / release-checklist.md
│   └── go-observability-architecture.drawio   # 可编辑架构图
│
├── AGENTS.md                    # 仓库开发守则（真源之一；防漂移硬规则）
├── README.md                    # 项目首页（真源之一）
├── CHANGELOG.md / LICENSE / SECURITY.md / SUPPORT.md / CONTRIBUTING.md / CODE_OF_CONDUCT.md
├── go.mod / go.sum              # Go 1.26；OTel 依赖集中在装配层
└── log/ 测试文件                # 与 log/ 包同目录的黑盒/单元测试（见上）
```

> `logs/` 与 `example/logs/` 是示例运行产物（已被 `.gitignore` 忽略），不属于源码。

## 4. 核心数据流

### 4.1 正常事件写出（`Logger.Emit` 管线）

```text
业务代码构造类型化事件（EventPayload + EventMetadata）
  -> 归一化 normalize.eventAttrs：metadata + payload 合并为扁平 attrs，过滤空键与保留键
  -> mergeBaseMetadata：用 WithBaseMetadata 补全缺失公共字段（不覆盖已设置值）
  -> Masker 脱敏（可选，FieldMasker 递归处理）
  -> Sampler 采样判定（可选，返回 false 直接丢弃，后续不再执行）
  -> Writer.Write 写出
       ├─ file/stdout：扁平 JSON 行 / stdoutlog（保留 timestamp/level/trace_id/span_id/event.name 运营键）
       └─ otlp：attrkv.Record 映射 LogRecord 顶层字段，trace/span 由 ctx span context 自动关联
  -> 写入失败不返回业务代码，只交给 ErrorHandler 观察
```

- `msg` 即 `event_type`；`attrs` 是完整扁平字段（公共 metadata + payload）。
- Logger 构造后配置只读，无需加锁；`Writer` / `Sampler` / `Masker` 需自行满足并发契约（服务端 Writer 必须支持多 goroutine 并发写）。

### 4.2 错误投影（`errs` → 事件）

```text
业务/中间件产生 error
  -> errs 建模：BizError（validation/business，预期内拒绝）或 SystemError（system，非预期故障）
  -> log.EventFromError(eventName, err, md) 沿错误链提取 AppError 并按 Kind 分派：
       ├─ KindValidation / KindBusiness → BusinessEvent（Result=failed，Level=WARN）
       ├─ KindSystem / 普通 error → ErrorEvent（Result=error，Level=LevelOf(err)）
       └─ StackMust 类别在 errs.NewSystem 构造点自动采集堆栈（runtime/db/redis/mq/http 前缀，
           runtime.context_cancelled 降为 optional 不自动采集）；投影仅渲染已采集的 StackTrace；
           默认策略可用 errs.SetStackPolicy 按 error.type 前缀覆盖（最长前缀优先，空=库默认）
  -> 进入 4.1 同一条 Emit 管线写出
```

错误事件名必须由调用方从 `types.go` 常量注册表传入，`EventFromError` 不自动派生；Trace/Span 由调用方从 span context 提取后填入 `md`。Gin 接入时，显式业务/系统错误由 `middleware/errresp` 在链尾统一收口（`c.Errors` + `Abort`），panic 仍由 `middleware/recover` 处理，二者组合时一个请求的错误事件唯一；经 InputGuard 注入的安全/审计事件可与错误事件并存（见 4.3）。

### 4.3 非法输入的安全/审计事件（方案 D，ADR-0007）

非法输入穿透校验触发系统错误时，错误收口（errresp / recover）仍只投影一条
`ErrorEvent`（错误出口唯一）；接入方在 handler 用 `httperr.WithInputSummary` 把
`InputSummary`（app.input_field / app.input_hash / app.input_truncated）挂到 ctx，
并配置 `InputGuard`——收口在写出 ErrorEvent 后调用 guard，按风险分级/命中规则补发
`SecurityEvent`（安全聚合 / SIEM）与 `AuditEvent`（合规审计），与错误事件并存，
共享同一 ctx（trace/span 自动关联）。输入只记 app.* 摘要，不落原始 body（配合 FieldMasker）。

除错误路径的 `InputGuard` 外，Security / Audit 事件另有独立中间件（与 access / error 同级）：
`ginmw.SecurityLog` / `httpmw.SecurityLog`（经 `SetSecurity` 挂载安全判定载荷后链尾写出
SecurityEvent，缺省 WARN）与 `ginmw.AuditLog` / `httpmw.AuditLog`（经 `SetAudit` 挂载审计
载荷后链尾写出 AuditEvent，缺省 INFO）。两者都从 ctx 关联 trace/span，未挂载载荷时直接放行。

## 5. 双投影：同一事件两种出口

| 字段 | file / stdout（运营投影） | OTLP（规范投影） |
| --- | --- | --- |
| `timestamp` | 扁平键 | `LogRecord.Timestamp` |
| `level` | 扁平键 | `SeverityNumber` + `SeverityText` |
| `event.name` | 扁平键 | `LogRecord.EventName` 顶层字段 |
| `trace_id` / `span_id` | 扁平键 | 由 ctx 的 span context 自动关联，**不写属性** |
| 其余 attrs | 扁平字段 | LogRecord Attributes |

顶层字段映射集中在 `internal/attrkv.Record`（唯一核心映射层）。**不要**为 OTLP 把顶层字段塞回属性，也**不要**为 file 把属性拆掉；两边共享同一份归一化 attrs。

file/stdout 的扁平投影按**固定字段顺序**输出：`timestamp` → `level` → `msg` → `trace_id`/`span_id`/`request_id`/`latency_ms` → `event.name` → 其余事件字段 → `app.result` 收尾；跨事件保持一致，避免同一字段（尤其 `app.result`）在不同日志里相对位置漂移。由 `writer/file` 实现，`example/blackbox` 测试锁定。

## 6. 关键设计决策与不可违反边界

1. **三段式事件名**：`类别.模块.操作`（如 `access.http.request`），必须经 `Validate`；框架级事件登记在 `types.go`，领域 `business.*` 由接入方自建注册表（见 `example/mall`），禁止在中间件/生产埋点里散落手写字符串。
2. **semconv 1.41.0 键名**：路径用 `url.path`（不是 `http.request.path`）；代码位置用 `code.function.name` / `code.file.path` / `code.line.number`。
3. **顶层映射集中**：新增保留键必须同时更新 `attrkv.recordAttrKeys` 与 `normalize.reservedKeys`，保证 OTLP 剥离与 file 保留一致。
4. **公共字段登记**：核心公共字段必须在 `keys.go` 登记；vendor 一律 `app.*`；领域专属键（`order_id` 等）由接入方自建，不进核心 `keys.go`。
5. **零值省略**：字符串/数值零值省略；布尔不省略（`false` 对 `retryable` / `result` 语义明确）。
6. **samber 边界**：samber 生态只允许出现在 `example/` 与 `docs/samber-comparison.md`，核心包保持零外部依赖。
7. **采样边界**：默认 `TraceSampleRatio=0.1` 是 SDK 头部采样——未选中的 trace 不会被导出，Collector 看不到，也无法通过 `tail_sampling` 恢复。需要 Collector 按错误/延迟决定保留时，SDK 应导出完整 trace（通常 `1.0`），再在 Collector 执行尾部采样，并评估吞吐、费用与敏感数据风险。

## 7. 三信号装配（`telemetry`）

- `Setup`：装配并全局安装 Trace（头部采样 + 批量 5s/512）、Metric（PeriodicReader 15s）、Log（批量 1s/512）Provider 与 W3C propagator；任一步失败回滚已创建资源，不留半初始化状态。
- `SetupFromEnvironment`：从 `OTEL_SDK_DISABLED` 读启用状态、`OTEL_EXPORTER_OTLP_ENDPOINT` 读 endpoint（缺省 `127.0.0.1:4317`）。
- `Providers.NewLogWriter`：复用 Setup 固化决策——显式配置 endpoint 且已启用 → OTLP Writer（共享 Resource/LoggerProvider）；否则写本地 JSONL（`file` Writer），离线可核对。
- `Providers.Shutdown`：进程退出前调用；顺序刻意先 log 再 metric 后 trace，保证日志携带的 span 上下文关联完整。

## 8. 扩展边界（接入方）

1. 在业务自己的 observability 包中声明 `EventName` 常量（`NewEventName` 或经 `Validate`），并写测试校验格式。
2. 使用 `BusinessPayload.ExtraAttrs` 注入领域属性（`app.*` 键），避免把领域字段加入核心注册表；canonical 键与保留键会被过滤。
3. 新增写出后端：在 `writer/` 下新建包，实现 `log.Writer`（明确并发语义，提供 `Close`）。
4. 公共 schema 或 API 改动必须同步 README、docs、示例与 CHANGELOG（改代码与改文档应在同一提交内完成）。

## 9. 代码 Review 检查点

| Review 焦点 | 看什么 | 验证方式 |
| --- | --- | --- |
| 核心零依赖 | `log/` 包的 import 是否仅标准库 | `go list -deps ./` 或 grep log 包 import |
| 事件名规范性 | `types.go` 注册表、`Validate` 实现与测试 | `go test ./...` |
| 键名 semconv | `keys.go` 与 `$GOMODCACHE/go.opentelemetry.io/otel@v1.45.0/semconv/v1.41.0` 对照 | 人工比对 |
| 归一化/保留键 | `normalize.go reservedKeys` 与 `attrkv.recordAttrKeys` 一致性 | `go test ./... -run Record` |
| 双投影形状 | `writer/otlp`、`writer/file`、`writer/stdout` 的输出测试 | `go test ./writer/...` |
| 错误投影 | `error_project.go`：Kind 分派、`LevelOf`、`StackRule`、值/指针/`%w` 链/nil | `go test ./... -run Error` |
| 采样/脱敏 | `sampler.go` 高价值保留/事件前缀全量、`masker.go` 递归脱敏与并发契约 | `go test ./... -run 'Sample|Mask'` |
| 错误收口 | `errresp.go`：Kind→状态码映射、system 响应不泄露、`Abort(nil)` 兜底、与 recover 不双写 | `go test ./middleware/errresp/...` |
| 并发安全 | Logger 构造后只读；Writer/ErrorHandler 多 goroutine 语义 | `go test -race ./...` |
| 三信号装配 | `telemetry.go` Setup 失败回滚、Shutdown 顺序、出口选择固化 | `go test ./telemetry/...` |

改动后的本地验证：`gofmt -w <改动文件>` → `go build ./...` → `go vet ./...` → `go test ./...`（本机低内存组合：`$env:GOMAXPROCS=1; $env:GOGC=30`）。

完整示例见 [`example/mall`](../example/mall/)；可编辑架构图见 [go-observability-architecture.drawio](go-observability-architecture.drawio)。
