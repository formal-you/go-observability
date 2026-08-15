# go-observability 博客系列（掘金版）

本目录是面向掘金发布的体系化文章源稿。文章与仓库示例同源，命令均从仓库根目录运行；
复制到掘金时请把相对链接替换为最终仓库链接或掘金文章链接。

## 系列定位

`go-observability` 不是又一个日志库，而是 `slog` 与 OpenTelemetry 之间的语义层：
把团队最容易失控的约定固化为可测试的 Go API。

## 目录与发布顺序

| # | 文章 | 主示例 | 核心问题 |
| --- | --- | --- | --- |
| 00 | [为什么需要语义层](00-why-semantic-layer.md) | `01_quickstart` | 从 `slog`/`zap` 痛点到稳定事件模型 |
| 01 | [六类类型化事件](01-typed-events.md) | `02_events` | 事件如何被查询、告警和治理 |
| 02 | [错误建模与投影](02-error-model.md) | `03_errors` | `ErrorKind`/`ErrorType`/`ErrorCode` 三层边界 |
| 03 | [日志治理：采样、脱敏、多出口](03-log-governance.md) | `04_sampler_masker`、`05_multiwriter` | 如何低成本保留高价值信号 |
| 04 | [三信号统一装配](04-telemetry-assembly.md) | `07_telemetry`、`09_gin` | Trace/Metric/Log 如何一次装配 |
| 05 | [框架中间件接入](05-framework-middleware.md) | `08_http`、`09_gin`、`10_grpc`、`11_kratos` | Gin/http/gRPC/kratos 如何接入 |
| 06 | [Security/Audit 审计留痕](06-security-audit.md) | `12_security_audit` | 安全与审计事件如何自动写成 |
| 07 | [JSONL 与 OTLP 双投影](07-jsonl-vs-otlp.md) | `14_otel_logs` | 同一事件如何适配本地排障与 OTel 后端 |
| 08 | [mall 端到端诊断故事](08-mall-end-to-end.md) | `mall`、`blackbox` | 用完整案例串起全部能力 |

## 每篇统一结构

1. 痛点：先讲不这样做会怎样；
2. 概念：引入术语，保留标准英文术语；
3. 动手：给出可运行命令与预期输出；
4. 输出解读：逐字段解释；
5. 最佳实践：可复制的建议；
6. 下一步：链接下一篇。

## 发布检查

- 命令来自仓库根目录，PowerShell 与 bash 各给一份；
- 引用只使用仓库公开路径，不写个人电脑路径；
- 第三方安装命令标注为“版本相关示例”，发布前核对版本；
- 所有代码片段与当前示例一致，发布前再跑一遍 `go test ./...`。
