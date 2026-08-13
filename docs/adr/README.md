# 架构决策记录（ADR）

本目录以 Architecture Decision Records（ADR）形式记录本仓库的关键技术决策：
「为什么这么做」以及被否掉的备选方案，供后续 Review 与演进时追溯。

## 约定

- 命名：`NNNN-短横线标题.md`，编号递增、不重用、不删改历史。
- 模板：背景（Context）→ 决策（Decision）→ 结果（Consequences），采用 Nygard ADR 风格。
- 状态：Proposed / Accepted / Deprecated / Superseded（被取代时在正文标注关联）。
- 定位：ADR 只记决策与理由；术语以 `CONTEXT.md` 为准，代码行为以代码为准。
  按 AGENTS.md 防漂移精神，改代码与改 ADR 应在同一提交内完成。

## 索引

| 编号 | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [0001](0001-errorcode-three-segment-format.md) | ErrorCode 采用三段式（服务/模块）.（场景/操作）.（结果/具体错误） | Accepted | 2026-08-11 |
| [0002](0002-errortype-low-cardinality-domain-reason.md) | ErrorType 映射 OTel error.type，保持低基数，采用 domain.reason 格式 | Superseded（ADR-0016） | 2026-08-11 |
| [0003](0003-move-root-log-package-to-log-subdir.md) | 核心 log 包迁移至 log/ 子目录，导入路径改为 .../go-observability/log | Accepted | 2026-08-11 |
| [0004](0004-eventtype-as-logrecord-body.md) | msg/event_type 映射 OTel LogRecord.Body，事件字段进属性 | Accepted | 2026-08-11 |
| [0005](0005-error-middleware-httperr-core.md) | 错误收口中间件抽取 httperr 契约核心 + 框架薄壳 | Accepted | 2026-08-11 |
| [0006](0006-middleware-framework-grouped.md) | 中间件按框架体系分组（gin / http / grpc / kratos） | Accepted | 2026-08-11 |
| [0007](0007-security-audit-events-for-bypassed-input.md) | 非法输入触发系统错误的事件记录策略（Security / Audit 并存，方案 D） | Accepted | 2026-08-11 |
| [0008](0008-security-audit-middleware.md) | Security / Audit 事件中间件（SecurityLog / AuditLog，拉取式 Decide/Describe） | Accepted | 2026-08-11 |
| [0009](0009-event-name-fact-and-error-code.md) | EventName 使用领域事实，BizError / SystemError 统一 error.code | Accepted | 2026-08-11 |
| [0010](0010-strict-error-construction.md) | 严格错误构造与 SCOPE.OPERATION.REASON / ErrorType 标准枚举 | Accepted | 2026-08-11 |
| [0011](0011-telemetry-runtime-explicit-output.md) | Telemetry Runtime 隔离全局状态并显式选择日志出口 | Accepted | 2026-08-11 |
| [0012](0012-subject-actor-context-and-privacy.md) | Subject / Actor 可信上下文与隐私边界 | Accepted | 2026-08-11 |
| [0013](0013-bounded-stack-policy.md) | 有界 StackPolicy 与路径治理 | Accepted | 2026-08-11 |
| [0014](0014-managed-writer-lifecycle.md) | 通过 ManagedWriter 统一 Writer 生命周期 | Accepted | 2026-08-12 |
| [0015](0015-error-registry.md) | Error Registry：error.code 到 error.type 的固定映射 | Accepted | 2026-08-12 |
| [0016](0016-errortype-otel-grpc-standard-enum.md) | ErrorType 复用 OTel/gRPC 标准枚举（跨模块） | Accepted | 2026-08-13 |
| [0017](0017-retain-app-result-sampling-policy.md) | 保留 app.result 结果列，Sampling/Retention 独立策略层 | Accepted | 2026-08-13 |
| [0018](0018-event-name-convention.md) | Event Name Convention：<event> 必须是注册的 Event Type | Accepted | 2026-08-13 |
| [0019](0019-errorcode-single-grammar-infra-scope.md) | error.code 单一文法：不分裂 reason/cause；SCOPE 失败面归属（INFRA.*），event.name/error.code 软对齐 | Accepted | 2026-08-13 |

## 模板

```markdown
# ADR-NNNN：标题

- 状态：Proposed / Accepted / Deprecated / Superseded
- 日期：YYYY-MM-DD
- 关联：ADR-XXXX（可选）

## 背景（Context）

...

## 决策（Decision）

...

## 结果（Consequences）

...
```
