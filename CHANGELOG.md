# Changelog

本项目按 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 组织内容，并遵循 [语义化版本](https://semver.org/lang/zh-CN/)。这里汇总对使用方有影响的版本变化；每次 Git 提交的具体内容请查看 `git log`。

## [Unreleased]

### Added

- 六类类型化事件、`Logger` / `Writer` 接口，以及 JSONL、stdout、OTLP Writer。
- `AccessPayload` 新增 `RPCInfo`（semconv `rpc.*`）与框架级事件名 `access.rpc.request` / `error.rpc.request`，支持 gRPC 传输层访问/错误事件。
- `errresp` 与 `recover` 中间件新增 `ResponseProjector` 配置：响应体与状态码可注入（默认保持扁平 `{code,message,request_id?}`），接入方可按自身 HTTP 契约投影。
- 新增 `middleware/nethttp`：net/http 版统一错误收口（`ErrorResponse`/`Recover`/`SetError`，支持 `ResponseProjector`；Logger 为 nil 时只渲染不写事件）。
- 新增 `middleware/metrics`：HTTP（net/http/Gin）与 gRPC 服务器指标中间件（`http.server.request.duration` / `rpc.server.duration`，semconv 1.41.0，默认全局 Meter，可注入）。
- 新增 `middleware/trace`：HTTP（net/http/Gin）与 gRPC 服务器链路中间件（server span 注入 request context，日志事件自动关联 trace_id/span_id，semconv 1.41.0，默认全局 Tracer，可注入）。
- `middleware/trace` 补全链路能力：HTTP/Gin/gRPC 入口提取上游传播上下文（traceparent/tracestate，接续调用方链路），新增 `InjectHTTPHeaders` / `InjectGRPCMetadata` 出口注入 helper。
- `errs` 错误分类、错误事件投影、Gin access、recover 与统一错误收口（`errresp`）中间件，以及框架级事件名 `error.http.request`。
- 公开 `telemetry` 包，提供 Trace、Metric、Log Provider 装配、环境变量入口和统一关闭。
- `ResultKeepSampler`、`FieldMasker` 与写入错误回调。
- 新增 `EventKeepSampler`：按 `event.name` 前缀全量保留（business./error./security./audit./probe.），其余事件委托 Fallback（如 `ResultKeepSampler`）采样——落地"业务全量 + 访问采样"策略。
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
- Error / Security / Audit payload 新增 `ExtraAttrs`（canonical 键守卫 + 保留键过滤）；新增框架级事件名 `security.input.anomaly` / `audit.input.anomaly` 与键 `app.input_field` / `app.input_hash` / `app.input_truncated`。

### Changed
- 升级 OpenTelemetry 依赖至 otel v1.45.0 / log v0.21.0（破坏性 API：log 包移除 `Value`/`KeyValue`，`attrkv` 迁移到 `attribute` 包）；Go 要求升至 1.26。

- `errs` 堆栈策略可配置：新增 `errs.SetStackPolicy`，使用方可按 `error.type` 前缀覆盖默认策略（最长前缀优先，空 map = 库内置默认）；`NewSystem` 构造采集与 `error_project` 事件渲染均跟随同一策略。
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
