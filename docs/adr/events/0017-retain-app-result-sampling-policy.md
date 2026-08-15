# ADR-0017：保留 app.result 结果列，Sampling/Retention 独立策略层

- 状态：Accepted
- 日期：2026-08-13
- 关联：ADR-0004（msg/EventType → Body）、ADR-0009（EventName/ErrorCode）、ADR-0016（ErrorType 标准枚举）

## 背景（Context）

曾评估把跨事件结果列更名 `event.outcome` 或整体移除、失败语义全交给 `error.type`。
移除的代价是失去跨事件通用的「一行式」结果过滤与采样锚点（access/security/audit/probe 没有
error.type 字段）；OTel semconv 1.41.0 也没有标准 `event.outcome` / `event.type` 键。
最终决定保留 vendor 结果列，并把采样/保留明确为独立策略层。

## 决策（Decision）

- 保留 `Result` 与 `app.result`（vendor 键，跨事件通用业务结果列），不做 `event.outcome`
  改名、不移除。
- 失败语义仍由 `error.type`（OTel/gRPC 标准枚举）+ `error.code`（SCOPE.OPERATION.REASON）
  表达；`app.result` 是直接过滤/采样用的视图列，不替代 `error.type`。
- 采样/保留是独立层（Sampling/Retention Policy）：高价值失败/异常事件（failed/error/
  blocked/denied）SHOULD be retained by the telemetry pipeline；sampling policy SHOULD
  prioritize or guarantee retention where operationally required；不把采样策略编码进
  `event.name` / `error.type` 定义。
- 事件模型三层：① Event Semantics（event.name 唯一标识 Event Structure）
  ② Error Semantics（error.type + error.code）③ Sampling / Retention。

## 结果（Consequences）

- `app.result` 继续作为跨事件结果列；采样器措辞从「强制保留」改为「高优先级保留（SHOULD）」。
- 文档（CONTEXT.md / security.md / configuration.md / README / 示例配置）同步 SHOULD 口径。
