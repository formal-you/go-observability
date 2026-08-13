# Changelog

本项目按 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 组织内容，并遵循 [语义化版本](https://semver.org/lang/zh-CN/)。这里汇总对使用方有影响的版本变化；每次 Git 提交的具体内容请查看 `git log`。

## [Unreleased]

### Changed

- Gin `Abort(nil)` 的固定系统错误改为包初始化时构造并验证，请求路径只复用已验证错误，避免在请求处理中触发库内固定契约构造失败。
- ErrorType 由 `domain.reason` 自定义词表迁移为 OTel/gRPC 标准枚举（gRPC canonical code，跨模块闭合枚举）：旧常量移除，映射如 `db.query_timeout → DEADLINE_EXCEEDED`、`db.connection_error → UNAVAILABLE`、`runtime.panic → INTERNAL`、`business.* → FAILED_PRECONDITION`；`SetStackPolicy` 改为按精确 code 覆盖。
- 框架不再提供泛化错误事件名（ADR-0018 / 方案 C）：移除 `http.request.failed` / `http.request.rejected` / `rpc.request.failed` / `database.query.timed_out` 常量；HTTP/Gin/Kratos 错误出口必须由接入方经 `EventName` / `EventNameResolver` 提供具体事实名，`EventFromError` 对事件名做 `EventNamePattern` 正则校验（非法 panic）。
- `error.code` 与 `event.name` 增加正则约束：`errs.ErrorCodePattern`（SCOPE.OPERATION.REASON）与 `log.EventNamePattern`（<domain>.<subject>.<event>），`Validate` 使用正则校验；EventName 注释明确 `<event>` 必须是注册的 Event Type、非自由文本、非生命周期 Stage。
- `error.code` 明确单一文法 `SCOPE.OPERATION.REASON`（ADR-0019）：第三段统一 REASON、不分裂 cause/reason；SCOPE 命名空间新增统一基础设施 `INFRA`（OPERATION 承担组件名，如 `INFRA.REDIS.UNAVAILABLE`），业务/系统区分由 error.type 承担，依赖名继续由 `app.upstream_service` 承载；黑盒新增 INFRA 系统错误示例。
- `error.code` SCOPE 归属由失败面决定（ADR-0019 补充）：业务/校验拒绝用业务模块，基础设施故障（DB/缓存/MQ/网络）用 `INFRA.*`（OPERATION=组件）；INTERNAL/panic 不承载 error.code；event.name 与 error.code 前两段软对齐（SHOULD，INFRA 与业务事件名不同源）。黑盒系统错误码迁移：`USER.ROLE_UPDATE.DB_TIMEOUT`→`INFRA.MYSQL.QUERY_TIMEOUT`、`MESSAGING.PUBLISH.DEADLINE_EXCEEDED`→`INFRA.MQ.PUBLISH_TIMEOUT`，`LOCK.ACQUIRE.CONFLICT`（并发冲突，非基础设施）保留领域 SCOPE。
- 保留 `Result` / `app.result`
- Writer 首个参数由 `msg` 改名为 `eventType`；file/stdout JSONL 输出键 `msg` 改为 `type`（粗分类列），`type` 加入保留键防 payload 覆盖；OTLP 仍映射 LogRecord.Body。
（不做 event.outcome 改名/移除）；采样保留措辞改为 SHOULD：高价值失败/异常事件由 Sampling/Retention Policy 高优先级保留（SHOULD be retained，操作上需要时保证保留），不编码进 event.name / error.type 语义。
- 修正 ADR 与当前设计的一致性：0001 关联/事件名术语、0004 备选方案 C 标记已采纳、0009 的 msg→type、0010 的 ErrorType 文法改为标准枚举（ADR-0016）、0013 堆栈覆盖改精确 code 与 INTERNAL 保持 must、0018 的 type 术语、README 索引。
- 新增 ADR-0018（Event Name Convention）与 ADR-0017（保留 app.result / Sampling/Retention 独立层），并把三层事件模型（Event/Error/Sampling）同步到 README、architecture、otel-logs 等文档。

### Added

- 新增 Error Registry：`errs.RegisterErrorCode` 注册 ErrorCode→ErrorType 固定映射（多对一），`ErrorCode.RegisteredErrorType` 反查；严格构造器对已注册码强制类型一致，未注册码保持既有行为。
- 新增 `log.ManagedWriter`、`log.ManageWriter` 与托管 `MultiWriter`；`telemetry.Runtime.NewWriter` 现在返回具备幂等关闭能力的 Writer，保留仅实现 `Write` 的旧 Adapter 兼容性。
- 新增严格错误值验证与配置式构造器：ErrorCode 使用 `SCOPE.OPERATION.REASON`，ErrorType 复用 OTel/gRPC 标准枚举（gRPC canonical code），并支持保留 cause 链。
- 六类类型化事件、`Logger` / `Writer` 接口，以及 JSONL、stdout、OTLP Writer。
- `AccessPayload` 新增 `RPCInfo`（semconv `rpc.*`）与框架级事件名 `rpc.request.completed`，支持 gRPC 传输层访问事件。
- `errresp` 与 `recover` 中间件新增 `ResponseProjector` 配置：响应体与状态码可注入（默认保持扁平 `{code,message,request_id?}`），接入方可按自身 HTTP 契约投影。
- 新增 `middleware/nethttp`：net/http 版统一错误收口（`ErrorResponse`/`Recover`/`SetError`，支持 `ResponseProjector`；Logger 为 nil 时只渲染不写事件）。
- 新增 `middleware/metrics`：HTTP（net/http/Gin）与 gRPC 服务器指标中间件（`http.server.request.duration` / `rpc.server.duration`，semconv 1.41.0，默认全局 Meter，可注入）。
- 新增 `middleware/trace`：HTTP（net/http/Gin）与 gRPC 服务器链路中间件（server span 注入 request context，日志事件自动关联 trace_id/span_id，semconv 1.41.0，默认全局 Tracer，可注入）。
- `middleware/trace` 补全链路能力：HTTP/Gin/gRPC 入口提取上游传播上下文（traceparent/tracestate，接续调用方链路），新增 `InjectHTTPHeaders` / `InjectGRPCMetadata` 出口注入 helper。
- `errs` 错误分类、错误事件投影、Gin access、recover 与统一错误收口（`errresp`）中间件，以及框架级 HTTP 请求事实事件名。
- 公开 `telemetry` 包，提供 Trace、Metric、Log Provider 装配、环境变量入口和统一关闭。
- 新增 `telemetry.SetupFile` 与 file Writer 服务元数据选项：无需 Collector 即可生成可关联的本地 trace/span，并在每条 JSONL 中写入规范 Resource 身份和 timestamp。
- file Writer 新增大小轮转、备份数量、保留天数与压缩选项；Logger 新增 `WithMinLevel` 最低级别过滤。
- blackbox 新增可实际读取的 YAML 配置模板，覆盖服务身份、最低级别、输出路径、覆盖策略、文件轮转和 OTLP endpoint。
- `ResultKeepSampler`、`FieldMasker` 与写入错误回调。
- 新增 `EventTypeKeepSampler`：按 `msg` / EventType 全量保留业务、错误、安全、审计和探测事件，其余事件委托 Fallback（如 `ResultKeepSampler`）采样；`EventKeepSampler` 保留用于领域前缀与旧配置兼容。
- 新增 `IdentityContext` / `IdentityExtractor`：可信 Subject / Actor 上下文自动补全，用户标识输出 semconv `user.id`；新增手机号、证件号、Cookie 和 request body 等默认敏感键。
- 新增 `errs.StackConfig`：提供 64 KiB 开发 / 16 KiB 生产 profile、路径策略、稳定截断标记与 panic 保护。
- `telemetry.Config` 支持注入公开 `TraceExporter` / `MetricReader`、OTLP 日志有界队列和 `Runtime.Stats` exporter 错误计数。
- 新增 `CONTEXT.md` 项目术语表：统一「采样」「批量导出间隔」「事件模型」及 OTel Trace / Metric / Log / Error 专业术语，避免沟通误导。
- `TraceExtractor` 接线：新增 `WithTraceExtractor` 与 `middleware/trace.NewTraceExtractor`，事件未显式携带 trace_id/span_id 时自动补全（不覆盖已设值）。
- 新增采样器构造器 `NewResultKeepSampler` / `NewEventKeepSampler`：非法入参构造期 panic，消除零值陷阱。
- 新增 `MultiWriter`（`NewMultiWriter`）：组合多个 Writer 多出口输出，错误聚合不阻断其余 Writer。
- `net/http`、Gin、指标、领域事件和 samber 对照示例。
- 本地 LGTM 参考栈、Collector 配置与分信号管线模板。
- 中文配置、安全、架构、贡献和发布检查文档。
- CI 检查与核心组件测试。
- 中文 Issue 表单、PR 模板、Dependabot 与首次发布检查清单。

- 新增 `middleware/kratos`：go-kratos v3 传输层适配——`ErrorEncoder`（HTTP 错误编码）与 `GRPCErrorMapper`（gRPC status 映射，reason/error.type 写入 `errdetails.ErrorInfo`），errs.AppError / kratos 原生错误双识别、system 与普通错误不透传内部细节；`ErrorLog` 错误事件日志 filter 复用 `log.EventFromError`。

- 新增 `SecurityLog` / `AuditLog` 中间件（gin + net/http）：`Decide` / `Describe` 拉取式——认证/授权中间件提供判定，链尾自动写出 SecurityEvent / AuditEvent（缺省 WARN / INFO，`Config.Level` 可覆盖）；`SetSecurity` / `SetAudit` 保留为深层代码判定的回退路径。
- 新增错误路径安全/审计注入点：`httperr.InputGuard` 与 `InputSummary`（`httperr.WithInputSummary`）——非法输入穿透校验触发系统错误时错误事件唯一，可按应用规则补发 Security / Audit 事件（ADR-0007 方案 D）。
- Error / Security / Audit payload 新增 `ExtraAttrs`（canonical 键守卫 + 保留键过滤）；新增框架级事实名 `input.threat.detected` / `input.anomaly.recorded` 与键 `app.input_field` / `app.input_hash` / `app.input_truncated`。

### Changed
- 收紧 `telemetry.Runtime` 的推荐公共表面：`Resource()` / `LoggerProvider()` 标记 Deprecated 并移入兼容层；仓库内部与正式黑盒改用 `Tracer`、`Meter`、`NewWriter`、`Stats` 和生命周期方法，不再探测或持有 Runtime 内部 Provider。
- `telemetry` 新增独立 `Runtime`、显式 `LogOutput` 与 `WriterConfig`；构造不再修改 OTel 全局状态，需显式 `InstallGlobal`，旧 `Setup*` 入口标记 Deprecated。
- 仓库生产示例和正式黑盒迁移到严格错误构造器；旧错误构造器与 SystemOption 保留为 Deprecated 兼容入口。
- `event.name` 从重复 `msg` 的「类别.模块.操作」调整为「领域.对象.事实」，并禁止六类 EventType 作为首段；HTTP 错误出口按错误 Kind 默认选择 `http.request.rejected` / `http.request.failed`。
- BizError 与 SystemError 的稳定具体错误码统一投影到 `error.code`；旧 `app.business_code` / `app.operation` 不再输出，旧 Go 字段仅保留源码兼容输入。
- 默认运维建议改为 HTTP AccessEvent 全量保留（健康检查用 `SkipPaths` 排除）；成功 access 概率采样保留为使用方显式选择，并明确会放弃完整的跨事件 access 关联。
- Resource 输出键修正为 `service.instance.id` / `deployment.environment.name`；`Environment` 默认值统一为 `development`。
- `Subject.UserID` 的新事件输出从 `app.user_id` 迁移到 semconv `user.id`；旧常量保留为 Deprecated 兼容输入。
- 升级 OpenTelemetry 依赖至 otel v1.45.0 / log v0.21.0（破坏性 API：log 包移除 `Value`/`KeyValue`，`attrkv` 迁移到 `attribute` 包）；Go 要求升至 1.26。

- `errs` 堆栈策略可配置：新增 `errs.SetStackPolicy`，使用方可按 `error.type` 精确 code 覆盖默认策略（空 map = 库内置默认）；`NewSystem` 构造采集与 `error_project` 事件渲染均跟随同一策略。
- 项目字段前缀统一为 `app.*`；电商等领域事件从核心包移至使用方示例。
- OpenTelemetry 属性键对齐 Semantic Conventions 1.41.0。
- file/stdout 与 OTLP 使用各自适合的字段投影，OTLP 顶层承载时间、严重级别、EventName 和 span context。
- 文档不再引用本机目录或仓库外设计资料，并明确首次公开发布前的验证门禁。
- `internal/telemetry` 提升为使用方可直接导入的公开 `telemetry` 包。
- `EventFromError` / `LevelOf` 接受标准 `error`，支持 `%w` 包装链和普通错误兜底。
- `DefaultSensitiveKeys` 从可变全局切片改为返回副本的函数，默认采样随机源不再使用固定种子。
- 开发工具链升级至 Go 1.25.12，gRPC 升级至 1.83.0；同步升级 `x/crypto`、`x/net`、`x/sys` 与 `x/text` 间接依赖。
- 移除进程级 `SeedSampleRand`；如需可重复测试，请通过 `ResultKeepSampler.RandFloat64` 注入随机源。

### Fixed

- 黑盒样例改用真实 OTel span + Gin 请求验证 access/business/error/security/audit 关联，并修复重复运行追加旧 JSONL、timestamp 键序断言与 Gin panic 缺 AccessEvent。
- OTLP LogRecord 顶层字段不再重复写入 attributes。
- 测试使用临时目录并关闭 Writer，避免仓库内残留测试产物。
- 修正文档中的 JSONL 输出路径、示例运行目录和头部/尾部采样说明。
- 修复关闭 OTLP Writer 时错误关闭外部共享 LoggerProvider 的问题。
- 修复自定义 OTLP `host:port` 被静默忽略或错误启用 TLS；裸地址按明文 gRPC 处理，非法 endpoint 返回显式错误。
- 修复 `Providers.NewLogWriter` 重读环境变量导致 Writer 与已创建 Provider 出口不一致的问题，出口现于 Setup 时固化。
- 修复审计命名 map、slice、group 在 OTLP 中被字符串化或丢失子键的问题。
- 修复 `FieldMasker` 未递归处理 map、slice 与 `LogValuer` 导致的嵌套敏感字段泄漏。
- 修复指针及 `%w` 包装错误丢失 retry、source、stack、upstream 信息，并为 typed-nil error 提供安全兜底。
- 阻止 `BusinessPayload.ExtraAttrs` 覆盖身份、资源、业务、代码位置、`event.name` 与 `app.result` 等 canonical 字段。
- `writer/file` JSONL 输出改为固定规范字段顺序（timestamp → level → msg → 链路/延迟 → event.name → 事件字段 → app.result 收尾），消除同一字段（尤其 `app.result`）在不同事件间相对位置漂移。

尚未创建首个版本标签，链接定义将在正式发布时补充。
