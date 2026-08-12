# ADR-0004：type/event_type（Writer 首个参数）映射 OTel LogRecord.Body（事件字段进属性）

- 状态：Accepted
- 日期：2026-08-11

## 背景（Context）

`log.Writer` 的接口形状继承自 slog：第一个位置参数是 `eventType string`，调用时无法省略。
同时 OTel Logs 数据模型提供多个顶层字段：Timestamp / Severity / SeverityText / Body / EventName。

本项目是「日志即事件」设计：事件字段进属性，不塞进正文。因此需要决定 `eventType` 这个槽位
放什么——粗分类 event_type？细名 event.name？还是可读消息？

## 决策（Decision）

- `Writer.Write(ctx, eventType, attrs...)` 的 `eventType` 参数承载**粗分类 event_type**
  （EventType：access / business / error / security / audit / probe）。
- OTLP 映射（`internal/attrkv.Record`）：
  - `eventType` → `LogRecord.Body`（超低基数，6 个稳定值）；
  - `event.name`（事实名，如 order.payment.succeeded）→ `LogRecord.EventName` 顶层字段；
  - 事件细节 → 属性（attrs）。
- file/stdout 扁平投影保留 `type`（=event_type）与 `event.name` 等键（双投影）。
- 明确不把高基数可读消息塞进 Body（CONTEXT.md 已固化：Body 写入 event_type，_Avoid_ 完整事件内容）。

## 理由

1. slog 形状必须有 eventType → 放稳定、低基数的值。
2. Body 超低基数 → 任何后端都能稳定分组/粗过滤，不依赖后端支持 EventName 字段。
3. 细名已由 OTel 原生 EventName 顶层字段承载，Body 不必重复。
4. 「日志即事件」：细节进 attrs，Body 不是人读消息。

## 备选方案与取舍

| 方案 | 描述 | 结论 |
| --- | --- | --- |
| A. msg 承载细名 event.name | Body = EventName，消除粗/细冗余 | 未采纳：与 OTel EventName 字段重复，Body 基数上升 |
| B. msg 承载可读消息（BusinessMessage 等） | Body = 自由文本 | 未采纳：高基数，违背「事件进属性」，与 attrs 消息字段重复 |
| C. 现状 + 参数改名 msg → eventType | 消除 slog 语义误导 | 可演进项：Writer 为库自定义接口，改名只破坏库内调用点（一次性） |

## 结果（Consequences）

- 正面：Body 低基数可聚合；细名走 OTel 原生字段；file/stdout 有稳定的粗分类列。
- 已知张力（已解决）：`eventType` 命名与 slog「消息」语义不一致，本次已把 Writer 首个参数由 `msg` 改名为 `eventType`（见 CHANGELOG）；event_type 是 event.name 的第一段，
  存在可推导冗余（见备选方案 C，可演进）。
- 现状：决策已固化于 CONTEXT.md（Body 定义）与 docs/architecture.md（type 即 event_type）。
