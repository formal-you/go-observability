# Changelog

本项目按 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 组织内容，并遵循 [语义化版本](https://semver.org/lang/zh-CN/)。这里汇总对使用方有影响的版本变化；每次 Git 提交的具体内容请查看 `git log`。

## [Unreleased]

### Added

- 六类类型化事件、`Logger` / `Writer` 接口，以及 JSONL、stdout、OTLP Writer。
- `AccessPayload` 新增 `RPCInfo`（semconv `rpc.*`）与框架级事件名 `access.rpc.request` / `error.rpc.request`，支持 gRPC 传输层访问/错误事件。
- `errs` 错误分类、错误事件投影、Gin access、recover 与统一错误收口（`errresp`）中间件，以及框架级事件名 `error.http.request`。
- 公开 `telemetry` 包，提供 Trace、Metric、Log Provider 装配、环境变量入口和统一关闭。
- `ResultKeepSampler`、`FieldMasker` 与写入错误回调。
- `net/http`、Gin、指标、领域事件和 samber 对照示例。
- 本地 LGTM 参考栈、Collector 配置与分信号管线模板。
- 中文配置、安全、架构、贡献和发布检查文档。
- CI 检查与核心组件测试。
- 中文 Issue 表单、PR 模板、Dependabot 与首次发布检查清单。

### Changed

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

尚未创建首个版本标签，链接定义将在正式发布时补充。
