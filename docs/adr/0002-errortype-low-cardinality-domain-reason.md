# ADR-0002：ErrorType 映射 OTel error.type，保持低基数，采用 domain.reason 格式

- 状态：Superseded（被 [ADR-0016](0016-errortype-otel-grpc-standard-enum.md) 取代）
- 日期：2026-08-11
- 关联：ADR-0001（ErrorCode 三段式）
- 注：ErrorType 改为复用 OTel/gRPC 标准枚举后，本文档的 domain.reason 决策已被取代；ErrorCode 与 ErrorType 的多对一关系在 ADR-0015/0016 中延续。

## 背景（Context）

需要一种低基数失败类别供 OTel Metrics（error.type）聚合与告警路由使用。若 ErrorType 跟随
ErrorCode 按业务模块细分，会产生上千个取值，Prometheus 维度（Labels）基数爆炸，Grafana 无法
聚合渲染。

OTel 语义约定原文：`error.type` SHOULD be predictable, and SHOULD have low cardinality；
规范只保留 `_OTHER` 一个 well-known 兜底值，其余允许自定义（须文档化枚举列表）。

## 决策（Decision）

ErrorType 映射 OTel error.type，保持低基数，采用 `domain.reason` 格式：

- 第一段 `domain` 为资源域/上下文：db / redis / mq / http / lock / idempotency / stock / data / runtime / validation / business。
- 第二段 `reason` 为具体失败原因（snake_case），如 `db.connection_error`、`business.stock_insufficient`。
- 系统侧为固定枚举（errs.go 常量，共 21 个）；`business.*` 为开放命名空间，调用方可自定义。
- 兜底：`error.unknown`（普通 error 无法归类时的稳定兜底）；不采用 OTel 的 `_OTHER`，
  以保持 `domain.reason` 格式统一。

与 ErrorCode 的关系是**多对一**：ErrorType 是比 ErrorCode 更高层的宏观分类，多个三段式
ErrorCode 归入同一个低基数 ErrorType；ErrorType 不是对 ErrorCode 的细化。

## 结果（Consequences）

- 正面：低基数，可安全作为 Metrics 维度与告警路由标签；同时驱动 `StackRule` 前缀堆栈策略
  （db./redis./mq./http./runtime. → StackMust 等），并作为 Business/Error 事件的
  error.type 字段投影（error_project.go）。
- 代价：业务侧需要自行维护 `business.*` 命名空间的受控列表（示例见 example/mall）。
- 兼容性：既有 21 个取值全部已符合 domain.reason，无值变更；本次仅统一注释与文档口径。