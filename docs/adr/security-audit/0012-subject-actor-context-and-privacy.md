# ADR-0012：Subject / Actor 可信上下文与隐私边界

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0007、ADR-0008、ADR-0011

## 背景（Context）

事件载荷已经包含 Subject 与 Actor，但调用方需要逐条复制用户和租户字段，错误事件也没有统一入口取得当前身份。事件属性可以由业务代码组装，因此不能把任意载荷中的身份值当作经过认证的上下文。`app.user_id` 还与 semconv 1.41.0 已定义的 `user.id` 重复。

## 决策（Decision）

- Subject 表示事件关联的用户与租户；Actor 表示执行安全或审计动作的用户与角色。二者不是 Metric 维度。
- 用户标识迁移为 semconv `user.id`；租户继续使用 vendor 键 `app.tenant_id`。Actor 继续使用 `app.actor_user_id` 与 `app.actor_role`，避免把被操作用户与操作者混为一人。
- `IdentityContext` 是统一的 Subject / Actor 上下文值。使用方通过 `IdentityExtractor` 注入可信认证结果；仓库提供基于 `context.Context` 的默认适配器。
- 提取器返回的非空字段优先于事件载荷中的同名字段。空字段不删除事件已经明确提供的值。只有 SecurityEvent 与 AuditEvent 接受 Actor 自动注入。
- `FieldMasker` 的默认敏感键覆盖凭证、Cookie、证件号、手机号和常见原始 request body 键，并递归处理结构化值。Logger 仍要求使用方显式启用 Masker，避免对已有数据形状做隐式修改；生产配置未启用脱敏属于接入错误。
- 原始 password、token、authorization、cookie、证件号、手机号和 request body 不得作为事件字段进入持久化日志。无法仅靠键名识别的自由文本与自定义类型必须在进入事件前净化。

## 结果（Consequences）

HTTP、后台任务和错误收口可以复用同一身份注入入口，可信认证上下文不会被业务事件属性覆盖。`app.user_id` 作为旧键常量保留并标记 Deprecated，但新事件只输出 `user.id`。身份字段只用于日志与 Trace 查询，现有 HTTP / gRPC Metrics 不增加用户或租户属性。
