# 架构与数据流（Architecture）

> 面向刚接手的新人：先看分层，再跟一条日志走一遍，最后知道"加一个事件要改哪"。
> 术语定义见 `../../observability-design/CONTEXT.md`，决策背景见 `DESIGN-DECISIONS.md`。

## 分层总览

| 层 | 包 | 一句话职责 | 依赖 |
| --- | --- | --- | --- |
| 核心 | 根包 `log`（types/keys/metadata/payload/events/normalize/log.go） | 类型化事件 + 归一化扁平 attrs；零外部依赖 | 仅标准库（slog/net/time/fmt/strings） |
| 错误体系 | `errs` | AppError 接口 + BizError/SystemError + error.type 枚举 + StackRule | 仅标准库 |
| 写出 | `writer/{otlp,stdout,file}` | 后端 Writer（OTLP gRPC / stdout / JSONL 文件） | OTel SDK + 根包 |
| 映射 | `internal/attrkv` | slog.Attr ↔ OTel KeyValue；LogRecord 顶层字段映射（唯一核心映射层） | OTel |
| 装配 | `internal/telemetry` | 三信号 provider + OTLP 导出 + A3 采样/频率 + A7 资源属性；B9 env 切换 | OTel SDK |
| 收口 | `middleware/{ginlog,recover}` | access 事件（otelgin trace 提取）；panic → ErrorEvent + 统一 500 | Gin + OTel trace |
| 演示 | `example` | Gin + telemetry + 各 writer 端到端 | 全部 |

## 一条日志的旅程（读这个最有收获）

```
业务代码构造事件（BusinessEvent{EventName, Subject, Result, ExtraAttrs}）
  → Logger.Emit(ctx, ev)
  → 归一化：ev.Attrs() 产出扁平 slog.Attr（双投影：semconv 运维键 + app.* 运营键）
  → Writer：file（JSONL）或 otlp（LogRecord，trace_id/span_id 由 ctx 自动关联）
```

关键点：

- **扁平输出**：事件结构体内聚组装，对外一律扁平 attrs，无 data 嵌套。
- **双投影**：运维面用 semconv 名（如 `http.request.method`），运营面 attr 键即扁平列（宽表）；共用一份归一化 attrs，业务只埋一次。
- **字段单一真源**：所有键在 `keys.go` 登记（semconv 名 + `app.*`）；事件名在 `types.go` 常量注册表（三段式 `类别.模块.操作`，`NewEventName`/`Validate` 校验）。
- **级别不靠猜**：错误事件的缺省级别由 `LevelOf(err)` 规则表推导（B3：validation/business=WARN，system 重试中=WARN、耗尽/不可重试=ERROR），显式 Level 优先。
- **出口不散落**：`telemetry.SetupFromEnvironment` + `(*Providers).NewLogWriter` 是唯一出口决策点（B9：endpoint env 空→JSONL、非空→OTLP）。

## 最小 OTel 概念铺垫（各 1-2 句，详读 CONTEXT.md）

- **trace / span**：一次请求的链路与其中一段；trace_id 贯穿日志/指标/链路。
- **log / metric**：日志（事件）与指标（数值聚合）；本组件以日志事件为源，指标由 B5 定义。
- **OTLP**：OpenTelemetry 的标准导出协议（gRPC/HTTP），collector 接收后分发到 Tempo/Loki/Mimir。
- **semconv**：OTel 语义约定——统一字段名（如 `http.request.method`），保证跨服务一致。

## 给新人的最小改动路径：新增一个业务事件

1. 框架级事件名登记核心 `types.go`；领域 `business.*` 在接入方包（`example/mall`）自建注册表。
2. 核心公共 `app.*` 键在 `keys.go`；领域专属键在接入方包登记，经 ExtraAttrs 注入。
3. 事件载荷：新字段用 `BusinessPayload.ExtraAttrs`（[]slog.Attr）承载，不新建结构体。
4. 同步 `../../observability-design/outline/B1-event-structs/detailed-design.md` §2 枚举。
5. 黑盒用例：`blackbox_log_test.go` 补 CASE（期望值来自 `../../observability-design/spec/acceptance.md` Oracle）。
6. 跑 `gofmt -w` + 低内存 `go build/vet/test ./...`，按 git 纪律 commit（见 `docs/workflow.md`）。
