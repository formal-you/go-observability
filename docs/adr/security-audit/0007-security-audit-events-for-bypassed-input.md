# ADR-0007：非法输入穿透校验触发系统错误的事件记录策略（SecurityEvent / AuditEvent 候选方案）

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0004（msg/event_type → LogRecord.Body）、CONTEXT.md（六类事件）、docs/architecture.md（错误投影 / 一个请求最多一个错误出口）

## 背景（Context）

用户非法输入绕过网关检测与业务系统校验，最终触发**非预期系统错误**（KindSystem）时，如何记录事件：

1. 是否需要额外记录 BusinessEvent？（结论：不需要——系统错误投影为 ErrorEvent，一次失败一条事件）
2. 是否需要记录用户输入了什么参数/数据？
3. 是否引入 SecurityEvent / AuditEvent 承载安全 / 审计视角？

现状核实：

- `middleware/*`（trace / errresp / recover）只提取 HTTP 元数据（method / route / url.path / status / client.address / user_agent / latency），**不捕获请求 body 或参数**；
- log 包已有六类事件，`SecurityEvent` / `SecurityPayload` 与 `AuditEvent` / `AuditPayload` 已存在；框架级 event.name 注册表在 `log/types.go`；`keys.go` 已有 `app.risk_score` 等键；
- 默认出口不是数据库：file / otlp / stdout 三种 Writer；输入数据的留存依赖后端保留策略（如 Loki / ClickHouse），不在库内。

## 决策（Decision）

**选定方案 D（按风险 / 资源分级组合）**：恒记 ErrorEvent；经中间件注入点 `InputGuard`
由接入方维护风险分级 / 命中规则，按需补发 `SecurityEvent` / `AuditEvent`；输入只记
`app.*` 摘要 + `FieldMasker`，不落原始 body。

已落地（与本文档同批提交）：
- `input.threat.detected` / `input.anomaly.recorded` 框架级事件名常量（log/types.go）；
- `app.input_field` / `app.input_hash` / `app.input_truncated` 键（log/keys.go）；
- Error / Security / Audit 三个 payload 的 `ExtraAttrs`（canonical 键守卫 + 保留键过滤）；
- `httperr.InputSummary` / `InputGuard` / `WithInputSummary` / `InputSummaryFromContext` /
  `EmitGuardEvents`（middleware/httperr）；
- gin 与 net/http 的 errresp + recover 均支持 `InputGuard` 注入点；
- example/blackbox 新增 security / audit 两条演示（9 → 11 条）与黑盒键序断言。

### 共同前提（任何方案都成立）

1. **不新增 BusinessEvent**：非法输入穿透后触发系统错误 → 投影为 `ErrorEvent`（KindSystem，result=error，带堆栈与重试上下文）；BusinessEvent 只用于校验层挡下的预期内拒绝（validation / business）。
2. **输入不落原始 body**：一律 `app.*` 摘要字段 + `FieldMasker` 脱敏；原始 payload 是高基数且可能含 PII / 凭证，直接落盘有合规风险。
3. 输入摘要作为 attrs 挂在错误 / 安全 / 审计事件上，而不是另发一条重复的错误事件。

### 方案 A（最小）：ErrorEvent + 输入摘要字段

- 系统错误照常记 `ErrorEvent`；在事件上附加 `app.input_field`（哪个字段非法）、`app.input_hash`（输入哈希，原文不落地）、`app.input_truncated`（截断前 N 字节）。
- 不引入 SecurityEvent / AuditEvent。
- 代码触点：`keys.go` 增加 `app.input_*` 键；错误收口处由调用方（或新增注入点）在事件 ExtraAttrs 注入输入摘要。
- 优点：改动最小、排障够用；不破坏「一个请求一个错误出口」。
- 缺点：非法输入不单独留痕，攻击特征无法按来源 IP / error.type 聚合；安全溯源依赖 trace 关联。

### 方案 B：ErrorEvent + SecurityEvent（风险 / 拦截双轨）

- 系统错误照常记 `ErrorEvent`（故障视角）；对「穿透校验的非法输入」额外发 `SecurityEvent`（如 event.name=`input.threat.detected`，result=denied/blocked，带 `app.input_*` 摘要、`app.risk_score`、来源 IP）。
- 代码触点：`types.go` 增加框架级 security 事件名常量；复用现有 `SecurityPayload`（SecurityEventType / FailureReason / ActionTaken / RiskScore / Result）；middleware 增加输入摘要注入点。
- 优点：安全视角独立、可按 risk_score / error.type / 来源 IP 聚合与告警，不污染故障事件；直连 SIEM。
- 缺点：一个请求可能出现两条事件，需要显式放宽「一个请求最多一个错误出口」为「错误出口唯一，安全事件可并存」；需要维护 security 事件名注册表与更高脱敏要求。

### 方案 C：ErrorEvent + AuditEvent（可追责审计）

- 系统错误照常记 `ErrorEvent`；对非法输入作为「敏感输入 / 异常操作」记 `AuditEvent`（如 event.name=`input.anomaly.recorded`），带 actor（用户）、target（资源）、before/after、`app.input_*`。
- 代码触点：`types.go` 增加框架级 audit 事件名常量；复用现有 `AuditPayload`（Action / Actor / TargetUserID / ChangedFields / Before / After / Reason / ApprovalID）；审计存储默认需防篡改（接入方责任）。
- 优点：满足合规「谁在什么时候提交了什么」的可追责要求。
- 缺点：AuditEvent 语义是「高权限操作 / 敏感资源变更」，普通用户输入是否算 audit 存在争议；保留与防篡改成本高于 Security。

### 方案 D（推荐组合）：按风险 / 资源分级触发

- 系统错误恒记 `ErrorEvent`；
- 输入风险低（普通参数错误被系统错误吸收）→ 只挂 `app.input_*` 摘要（= 方案 A）；
- 输入命中已知攻击特征 / 高风险（注入类、异常基数、来源 IP 黑名单）→ 加发 `SecurityEvent`（= 方案 B）；
- 输入涉及高权限 / 敏感资源变更 → 加发 `AuditEvent`（= 方案 C）。
- 代码触点：A/B/C 的并集；接入方维护「风险分级 / 命中规则」，库只提供事件类型与注入点。
- 优点：成本与价值匹配，粒度可控。
- 缺点：需要接入方维护分级规则，初始工作量最大。

## 候选方案对比

| 方案 | 记录内容 | 是否新增事件类型 | 代码触点 | 成本 | 适用场景 |
| --- | --- | --- | --- | --- | --- |
| A | ErrorEvent + app.input_* | 否 | keys.go + 注入点 | 低 | 只想排障、不想引安全/审计语义 |
| B | ErrorEvent + SecurityEvent | security 事件名常量 | types.go + 注入点 | 中 | 需要攻击聚合 / SIEM / 风控告警 |
| C | ErrorEvent + AuditEvent | audit 事件名常量 | types.go + 注入点 | 中高 | 合规审计、可追责留痕 |
| D | A/B/C 分级组合 | 上述并集 | 上述并集 + 分级规则 | 高 | 生产级、安全与合规都要 |

## 结果（Consequences）

- 方案 A / B / C 作为备选记录，未单独采纳（其能力已被 D 的组合覆盖）。
- 已显式放宽「一个请求最多一个错误出口」为「错误出口唯一，安全 / 审计事件可并存」，并同步
  `docs/architecture.md`、`middleware/gin/errresp.go` 注释与 `CONTEXT.md` 术语。
- 输入记录一律走 `app.*` 摘要 + `FieldMasker`，默认不落原始 body；后端保留策略由接入方负责。
- 选定后：新增事件名常量、示例（example）与黑盒断言测试，并回填本 ADR 状态为 Accepted 及最终方案。
