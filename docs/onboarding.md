# 新人引导（Onboarding）—— go-observability

> 你是刚接手本项目的 Go 开发者。本文给你 15 分钟定位 + 5 分钟跑通，然后指向该读的文档。
> 原则：本文只做索引与摘要，细节一律链接原文，避免多份文档漂移。

## 这是什么

**go-observability** 是商城后端（模块化单体）的可观测性层：一个基于 OpenTelemetry 语义约定的
Go 语义化日志组件（方案2「源即规范」）。它不只写日志——它产出**类型化领域事件**，并统一导出到
Grafana LGTM 栈（Tempo 链路 / Loki 日志 / Mimir 指标）。

- 核心 log 包**零外部依赖**（仅标准库），属性键直接对齐 OTel semconv 1.41.0 + `mall.*` vendor 命名空间。
- 接入面是 `log/slog` + 标准库 + OTel SDK（仅装配层），不绑定特定框架。

## 两个工作区（先认清，避免迷路）

| 工作区 | 路径 | 是什么 | git |
| --- | --- | --- | --- |
| 代码仓库 | `Desktop\商城\go-observability` | 组件实现（根包 log + errs/writers/telemetry/middleware） | ✅ git（每次改动 commit） |
| 设计工作区 | `Desktop\商城\observability-design` | 设计文档：术语表、决策记录、wayfinder 票、验收契约 | ❌ 非 git（改动即生效） |

> 本项目「**设计即文档**」：先定决策（grill-with-docs），再落代码；决策/术语变更即时写回设计工作区。
> 若只 clone 了代码仓库，设计区链接会失效——本机工作按两区并存对待。

## 阅读顺序（按需，别一次读完）

1. 本文件（定位 + 上手）
2. `../README.md`（特性 / 快速开始 / 包布局 / OTel 符合性）
3. `../../observability-design/CONTEXT.md`（领域术语，先认词）
4. `../../observability-design/DESIGN-DECISIONS.md`（A1-A7 / B1-B9 决策，看"为什么这么设计"）
5. 按任务再读：`docs/architecture.md`（代码走读）、`docs/workflow.md`（流程/规则）、`../AGENTS.md`（改代码前必读）

## 15 分钟上手

```powershell
cd Desktop\商城\go-observability
# 低内存组合（本机建议）
$env:GOMAXPROCS=1; $env:GOGC=30
# 跑通测试
go build ./...; go vet ./...; go test ./...
# 跑 example：默认写 JSONL 到 example/logs/events.jsonl
go run ./example
```

- 不设 `OTEL_EXPORTER_OTLP_ENDPOINT` → 本地 JSONL 快速核对（B9 dev 模式）；设置后 → OTLP（联调/生产）；`OTEL_SDK_DISABLED=true` → 全离线。
- 本机无 docker：`../observability/` 的 LGTM 参考栈只做 YAML/JSON 静态校验，不实际起容器。

## 文档索引

| 文档 | 用途 | 什么时候读 |
| --- | --- | --- |
| `docs/onboarding.md`（本文件） | 定位 + 上手 + 索引 | 第一天 |
| `README.md` | 特性 / 快速开始 / 包布局 | 第一天 |
| `docs/architecture.md` | 分层架构 + 一条日志的旅程 + 最小改动路径 | 开始读代码时 |
| `docs/workflow.md` | 开发流程（wayfinder→grill→prototype→implement）+ git/本机规则 | 开始干活时 |
| `AGENTS.md` | 防漂移硬规则（事件名注册表 / semconv 键 / 双投影） | 改代码前必读 |
| `docs/samber-comparison.md` | 与 samber slog 生态对比（为什么不自研全套） | 有人问"为什么不直接用 samber" |
| `observability/README.md` | LGTM 参考栈（docker-compose + Grafana） | 起本地栈时 |
| `../../observability-design/CONTEXT.md` | 领域术语表 | 遇到不懂的词 |
| `../../observability-design/DESIGN-DECISIONS.md` | A1-A7 / B1-B9 决策记录 | 想懂"为什么" |
| `../../observability-design/wayfinder/MAP.md` | 决策地图 + frontier（当前可认领票） | 想接下一张票 |
| `../../observability-design/spec/acceptance.md` | 黑盒验收契约（Oracle） | 写/理解黑盒测试时 |
