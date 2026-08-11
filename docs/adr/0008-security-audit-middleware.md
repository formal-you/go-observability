# ADR-0008：Security / Audit 事件中间件（SecurityLog / AuditLog，拉取式）

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0007（非法输入触发系统错误的安全/审计事件记录，方案 D）、ADR-0006（中间件按框架体系分组）、ADR-0005（httperr 契约核心）

## 背景（Context）

六类事件中 SecurityEvent / AuditEvent 只有类型与载荷，没有与 access / error 同级的中间件：
安全判定（认证 / 授权 / 风控命中）与审计留痕（高权限操作 / 敏感资源变更）需要在哪里、由谁记录？

接入方期望：认证 / 授权中间件在判定时完成日志记录，而不是在业务代码里手动构造载荷写日志。

## 决策（Decision）

- 新增 `SecurityLog` / `AuditLog` 链尾收口中间件（gin + net/http）：
  - `SecurityConfig.Decide`：认证 / 授权中间件提供的判定函数，链尾自动调用，返回非 nil 即写出
    SecurityEvent（缺省 WARN）；返回 nil 不记录。
  - `AuditConfig.Describe`：链尾（handler 完成后）组装 AuditPayload 并写出 AuditEvent（缺省 INFO）。
  - `SetSecurity` / `SetAudit` 保留为深层代码判定的回退路径（Decide / Describe 为空时生效）。
- 中间件只记录不拦截：401 / 403 等拦截由认证 / 授权中间件自行完成，SecurityLog 只负责把判定
  结果写成事件。
- 与 errresp 的 `InputGuard`（ADR-0007）互补：错误路径按风险补发用 InputGuard；显式安全判定 /
  审计留痕用 SecurityLog / AuditLog，二者不冲突。
- 元数据统一：链尾从 ctx 提取 trace / span（`httperr.EventMetadataFromContext`），可选
  `GetRequestID`；一次请求一条事件。
- 明确「错误出口唯一，安全 / 审计事件可并存」（同步放宽 ADR-0007 与 architecture.md 的表述）。

## 被否方案

- 认证 / 授权中间件直接 `logger.Emit`：每个认证中间件都要自行拼 metadata（trace / span /
  request_id），重复且易错；无法保证一次请求一条。
- 仅 Set 式（`SetSecurity` + 链尾读）：接入方仍需手动构造载荷调用 SetSecurity，不符合
  「认证中间件给结果」的直觉。

## 结果（Consequences）

- 正面：认证 / 授权中间件把判定逻辑放进 `Decide` 即自动记录；元数据一致；Set 式保留兼容。
- 代价：需要注册 SecurityLog / AuditLog 中间件；Decide / Describe 由接入方维护（风险分级、
  动作 / actor / 资源组装）。
- 兼容：既有 Set 式 API 与测试原样通过（回退路径）；gin + net/http 行为一致。
