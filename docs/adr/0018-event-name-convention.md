# ADR-0018：Event Name Convention——<event> 必须是注册的 Event Type

- 状态：Accepted
- 日期：2026-08-13
- 关联：ADR-0009（EventName/ErrorCode 分工）、ADR-0004（msg/EventType → Body）、ADR-0016（ErrorType 标准枚举）、ADR-0017（保留 app.result）

## 背景（Context）

改造方案要求把 event.name 的 `<event>` 段从「自由词」收紧为注册的 Event Type：
没有注册表时，团队会出现 payment.charge.failed / payment.charge.failure /
payment.charge.auth_failed 等近似命名，Grafana 无法判断是否同一事件。同时要防止把
Event Type 退化成 Operation Lifecycle Stage（INITIATED/COMPLETED/CANCELLED），
那会偏离 OTel Event 的核心模型——event.name 标识的是 Event Structure。

## 决策（Decision）

- event.name MUST use the form `<domain>.<subject>.<event>`；正则 `log.EventNamePattern`
  校验（每段小写字母/数字/下划线），首段禁止六类 EventType 前缀（粗分类由 type 承载）。
- `<event>` MUST 是注册的 Event Type：稳定语义发生、唯一标识一个 Event Structure，
  不是自由文本；MUST NOT 编码动态值、错误分类或 Operation Lifecycle State，除非该
  生命周期转换本身就是要记录的语义事件（如 http.request.completed）。
- Operation lifecycle 独立建模：经 Span 生命周期；确需显式阶段时另建 event.stage 字段。
- 不新增 `event.type` 字段：粗分类（六类）已由 msg / LogRecord.Body 承载（ADR-0004），
  OTel semconv 1.41.0 无标准 event.type 键，再加字段纯属重复。
- 框架级事件登记在 log/types.go（Event Type 注册表）；领域事件由接入方自建注册表
  （见 example/mall），禁止散落手写字符串。

## 结果（Consequences）

- event.name 可枚举、可校验、唯一标识 Event Structure；查询/告警不会因自由词歧义漂移。
- 生命周期阶段不再污染 event.name 语义；需要阶段时走 Span 或显式 event.stage。
- 与 Error Semantics 分层：失败/异常事件的 error.type（ADR-0016）+ error.code（SCOPE.OPERATION.REASON）独立表达。
