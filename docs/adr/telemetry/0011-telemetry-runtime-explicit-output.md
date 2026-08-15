# ADR-0011：Telemetry Runtime 隔离全局状态并显式选择日志出口

- 状态：Superseded（ADR-0020）
- 日期：2026-08-11
- 关联：ADR-0010

## 背景（Context）

旧版 `Setup` 同时创建 Provider、推断日志出口并安装 OTel 全局状态，导致构造过程有隐式副作用，也无法表达 stdout 与 none 出口。file-only 小单体还需要在不连接 Collector 的情况下生成可关联的本地 Trace ID。

## 决策（Decision）

新增 `Runtime` 与 `NewRuntime`。构造 Runtime 不修改全局 OTel Provider 或 propagator；使用方显式调用 `InstallGlobal`，并保存幂等恢复函数。`Config.LogOutput` 只接受 `file`、`otlp`、`stdout`、`none`，通过 `Runtime.NewWriter` 创建对应 Writer，配置不匹配时返回错误。

只有 `LogOutputOTLP` 创建 OTel Log exporter；Trace/Metric exporter 仍由 Runtime 的 OTLP endpoint 配置。`NewFileRuntime` 不创建任何 exporter，使用 `ParentBased(NeverSample())` 生成本地非 sampled trace/span。旧 `Setup*` 与 `NewLogWriter` 保留为 Deprecated 兼容层。

## 结果（Consequences）

Runtime 可独立测试、嵌套安装并按 LIFO 恢复全局状态。file/stdout/none 不会意外建立 OTLP 日志连接；OTLP Writer 复用 Runtime LoggerProvider 和 Resource。完整 Trace 树仍需 OTLP/Tempo，file-only 只提供日志中的链路标识。
