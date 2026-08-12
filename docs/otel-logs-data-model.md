# OTel Logs 顶层字段说明

> 版本基准：`go.opentelemetry.io/otel/log@v0.21.0` + `sdk/log@v0.21.0`（OTLP Logs 数据模型）。
> 本仓库对该数据模型的映射见文末「go-observability 映射」，并已记录于 ADR-0004（Body=event_type）。

## 1. OTel Logs 数据模型（spec / OTLP 层）顶层字段

LogRecord 在 OTLP 线格式中的顶层字段：

| OTLP 字段 | 类型 | 说明 |
| --- | --- | --- |
| `time_unix_nano` | uint64 | 事件发生时间（Timestamp） |
| `observed_time_unix_nano` | uint64 | 观测时间（ObservedTimestamp），通常由 SDK/Collector 打点 |
| `severity_number` | enum | 严重度数值（DEBUG/INFO/WARN/ERROR/…，1-24） |
| `severity_text` | string | 原始严重度文本（如 "INFO"） |
| `body` | AnyValue | 日志正文 |
| `attributes` | map | 结构化键值属性（高基数内容放这里） |
| `dropped_attributes_count` | uint32 | 因 limit 被丢弃的属性数 |
| `flags` | uint32 | TraceFlags（sampled 等） |
| `trace_id` / `span_id` | bytes | 关联的 Trace/Span 标识 |
| `event_name` | string | 事件名（非空时该记录被解释为「事件记录」） |

> 注：`Resource`（实体标识：service.name 等）与 `InstrumentationScope`（产生日志的库/组件）不在单个 LogRecord 内，而是由 SDK 在导出时附加到每条记录。

## 2. Go API 层：`otel/log.Record`（v0.21.0）

本仓库 `internal/attrkv.Record` 直接构造的就是这个结构：

| 字段 | Getter / Setter | 说明 |
| --- | --- | --- |
| `timestamp` | `Timestamp()` / `SetTimestamp` | 事件发生时间 |
| `observedTimestamp` | `ObservedTimestamp()` / `SetObservedTimestamp` | 观测时间 |
| `severity` | `Severity()` / `SetSeverity` | 严重度数值（`Severity` 枚举） |
| `severityText` | `SeverityText()` / `SetSeverityText` | 原始严重度文本 |
| `body` | `Body()` / `SetBody(attribute.Value)` | 正文（AnyValue） |
| `eventName` | `EventName()` / `SetEventName` | 事件名；**非空即事件记录**，应唯一标识该事件的属性/正文结构 |
| `err` | `Err()` / `SetErr` | 关联错误（Go 特有扩展字段） |
| attributes | `WalkAttributes` / `AddAttributes` | 属性集合（内联 5 个 + 溢出切片） |

## 3. Go SDK 层：`sdk/log.Record` 额外附加字段

API 层 Record 之外，SDK 在导出前附加：

| 字段 | 说明 |
| --- | --- |
| `traceID` / `spanID` / `traceFlags` | 从 ctx 的 span context 自动关联（`Logger.Emit(ctx)`） |
| `resource` | 采集实体（service.name / version / environment 等） |
| `scope` | 创建该 Logger 的 InstrumentationScope |
| `dropped` | 达到 limit 时丢弃的属性计数 |

## 4. go-observability 映射

| LogRecord 顶层字段 | 来源（本仓库） |
| --- | --- |
| `timestamp` / `observedTimestamp` | `attrkv.Record` 内 `time.Now()` |
| `severity` / `severityText` | `level` 属性（DEBUG/INFO/WARN/ERROR）映射 |
| `body` | **`msg` = event_type 粗分类**（access/business/error/security/audit/probe，见 ADR-0004） |
| `eventName` | `event.name` 属性（`<domain>.<subject>.<event>`，如 order.payment.succeeded） |
| attributes | 其余扁平字段（剥离 timestamp/level/event.name/trace_id/span_id 后） |
| `trace_id` / `span_id` / `traceFlags` | 由 sdk/log 从 ctx 的 span context 自动关联（不写属性） |
| `resource` / `scope` | `telemetry` 装配的 Provider / Logger 提供 |
| `err` | 本仓库未使用（错误信息走 attrs 与 Body 语义） |

## 5. 注意点

- **EventName 语义**：OTel 明确「非空 event_name 的记录被解释为事件记录」，且事件名应唯一标识事件的属性/正文结构。本仓库用 `event.name = <domain>.<subject>.<event>`（正则 `EventNamePattern` 校验）；`<event>` MUST 是注册的 Event Type（稳定语义发生，唯一标识 Event Structure），不是自由文本，也不是 Operation Lifecycle Stage（生命周期经 Span 建模）；`msg` 单独承载 access/business/error/security/audit/probe 粗分类。
- **Body 低基数**：Body 不放高基数自由文本，放 event_type 粗分类（决策见 ADR-0004）。
- **trace/span 不走属性**：trace_id/span_id 由 ctx 自动关联，双投影规则见 [架构说明](architecture.md#5-双投影同一事件两种出口)。
- **版本差异**：`event_name` 是较新的数据模型字段；早期 Collector/后端不支持时，Body 仍保留 event_type 作为兜底分组键。
