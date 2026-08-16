# ADR-0022：日志写出实现目录命名为 logwriter/

- 状态：Accepted
- 日期：2026-08-16
- 关联：ADR-0014（ManagedWriter 生命周期）、ADR-0020（按信号拆分配置）

## 背景（Context）

`writer/` 目录下只有 `file`、`stdout`、`otlp` 三个实现 `log.Writer` 的日志写出包；
Trace 与 Metric 没有项目自定义 Writer，而是直接使用 OTel SDK 的 `sdktrace.SpanExporter`
与 `sdkmetric.Reader`，并在 `telemetry` 层装配。目录名 `writer/` 会被误读为三信号统一
写出层，边界不够清晰。

## 决策（Decision）

- 将目录重命名为 `logwriter/`，明确该目录只承载 Log 信号的写出实现。
- 保留三个子包名不变：`file`、`stdout`、`otlp`。
- 公开导入路径从 `github.com/formal-you/go-observability/writer/{file,stdout,otlp}`
  改为 `github.com/formal-you/go-observability/logwriter/{file,stdout,otlp}`。
- 不为 Trace / Metric 新增项目自定义 Writer 抽象，继续使用 OTel SDK 原生接口。

## 结果（Consequences）

- 包边界与命名一致：`logwriter/` 只表达日志写出，装配层 `telemetry` 继续负责三信号。
- 旧公开导入路径失效，属于 v0.x 破坏性变更，使用方需全局替换导入路径；子包名不变。
- 后续新增日志后端统一在 `logwriter/` 下实现 `log.Writer`，避免再次扩散命名。