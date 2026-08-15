# 架构决策记录（ADR）

本目录以 Architecture Decision Records（ADR）形式记录本仓库的关键技术决策：
「为什么这么做」以及被否掉的备选方案，供后续 Review 与演进时追溯。

## 约定

- 命名：`NNNN-短横线标题.md`，编号递增、不重用；无后续影响的低价值/被取代决策可删除。
- 模板：背景（Context）→ 决策（Decision）→ 结果（Consequences），采用 Nygard ADR 风格。
- 状态：Proposed / Accepted / Deprecated / Superseded（被取代时在正文标注关联）。
- 定位：ADR 只记决策与理由；术语以 `CONTEXT.md` 为准，代码行为以代码为准。
  按 AGENTS.md 防漂移精神，改代码与改 ADR 应在同一提交内完成。

## 索引

ADR 按主题分目录存放，编号仍全局递增、不重用。

### 错误模型（errors）

| 编号 | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [0001](errors/0001-errorcode-three-segment-format.md) | ErrorCode 采用三段式（服务/模块）.（场景/操作）.（结果/具体错误） | Accepted | 2026-08-11 |
| [0010](errors/0010-strict-error-construction.md) | 严格错误构造与 SCOPE.OPERATION.REASON / ErrorType 标准枚举 | Accepted | 2026-08-11 |
| [0013](errors/0013-bounded-stack-policy.md) | 有界 StackPolicy 与路径治理 | Accepted | 2026-08-11 |
| [0015](errors/0015-error-registry.md) | Error Registry：error.code 到 error.type 的固定映射 | Accepted | 2026-08-12 |
| [0016](errors/0016-errortype-otel-grpc-standard-enum.md) | ErrorType 复用 OTel/gRPC 标准枚举（跨模块） | Accepted | 2026-08-13 |
| [0019](errors/0019-errorcode-single-grammar-infra-scope.md) | error.code 单一文法：不分裂 reason/cause；SCOPE 失败面归属（INFRA.*），event.name/error.code 软对齐 | Accepted | 2026-08-13 |
| [0021](errors/0021-panic-boundary-and-recover.md) | Panic 只用于启动期契约校验，Recover 是请求期最后保险 | Accepted | 2026-08-16 |

### 事件模型（events）

| 编号 | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [0004](events/0004-eventtype-as-logrecord-body.md) | type/event_type 映射 OTel LogRecord.Body，事件字段进属性 | Accepted | 2026-08-11 |
| [0009](events/0009-event-name-fact-and-error-code.md) | EventName 使用领域事实，BizError / SystemError 统一 error.code | Accepted | 2026-08-11 |
| [0017](events/0017-retain-app-result-sampling-policy.md) | 保留 app.result 结果列，Sampling/Retention 独立策略层 | Accepted | 2026-08-13 |
| [0018](events/0018-event-name-convention.md) | Event Name Convention：<event> 必须是注册的 Event Type | Accepted | 2026-08-13 |

### 安全与审计（security-audit）

| 编号 | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [0007](security-audit/0007-security-audit-events-for-bypassed-input.md) | 非法输入触发系统错误的事件记录策略（Security / Audit 并存，方案 D） | Accepted | 2026-08-11 |
| [0008](security-audit/0008-security-audit-middleware.md) | Security / Audit 事件中间件（SecurityLog / AuditLog，拉取式 Decide/Describe） | Accepted | 2026-08-11 |
| [0012](security-audit/0012-subject-actor-context-and-privacy.md) | Subject / Actor 可信上下文与隐私边界 | Accepted | 2026-08-11 |

### 中间件（middleware）

| 编号 | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [0005](middleware/0005-error-middleware-httperr-core.md) | 错误收口中间件抽取 httperr 契约核心 + 框架薄壳 | Accepted | 2026-08-11 |
| [0006](middleware/0006-middleware-framework-grouped.md) | 中间件按框架体系分组（gin / http / grpc / kratos） | Accepted | 2026-08-11 |

### 遥测装配（telemetry）

| 编号 | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [0014](telemetry/0014-managed-writer-lifecycle.md) | 通过 ManagedWriter 统一 Writer 生命周期 | Accepted | 2026-08-12 |
| [0020](telemetry/0020-telemetry-per-signal-config.md) | Telemetry 按信号拆分配置并移除兼容层 | Accepted | 2026-08-15 |


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
