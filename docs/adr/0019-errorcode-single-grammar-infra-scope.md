# ADR-0019：error.code 保持单一文法，SCOPE 纳入 INFRA 基础设施命名空间

- 状态：Accepted
- 日期：2026-08-13
- 关联：ADR-0001（ErrorCode 三段式）、ADR-0010（SCOPE.OPERATION.REASON 严格文法）、ADR-0015（Error Registry）、ADR-0016（ErrorType OTel/gRPC 标准枚举）、ADR-0018（Event Name Convention）

## 背景（Context）

讨论中提出「业务错误码 vs 系统错误码」两个概念：把 error.code 第三段按业务/系统分裂成
reason（业务）与 cause（系统）两套命名，或用依赖名（REDIS / MYSQL / KAFKA…）直接作为
SCOPE。两者都会让同一字段位置承载两套语义，读日志/写代码的人必须先判断「这是业务错误还是
系统错误」才能正确解读第三段——这正是此前否掉「EventName 第三段是阶段还是事实」时同一类
歧义。

同时系统错误码缺少明确的承载面：基础设施故障（缓存/数据库/MQ/网络）没有与业务模块平级的
SCOPE 命名空间。

## 决策（Decision）

- error.code 保持单一文法 `SCOPE.OPERATION.REASON`，第三段统一为 REASON，不分裂
  cause/reason 两套命名。
- 业务/系统（预期拒绝 vs 非预期故障）的区分由 error.type（OTel/gRPC 标准枚举，
  见 ADR-0016）承担，不是 error.code 段名的职责。
- SCOPE 命名空间分两类：
  - 业务模块：ORDER / PAYMENT / USER …（服务/模块边界）；
  - 统一基础设施命名空间 `INFRA`：第二段（OPERATION）承担组件名（REDIS / MYSQL /
    MONGODB / KAFKA …），第三段（REASON）承担具体故障，例如 `INFRA.REDIS.UNAVAILABLE`、
    `INFRA.MYSQL.CONNECTION_POOL_EXHAUSTED`、`INFRA.MQ.CONSUMER_LAG_EXCEEDED`。
- 不用依赖名直接作为 SCOPE（如 `REDIS.CONNECTION_POOL_EXHAUSTED`）：避免 SCOPE/OPERATION
  槽位两套读法与技术栈迁移（如 Redis→Valkey）导致的错误码改迁；依赖名由 `app.upstream_service`
  属性承载。
- 受众差异（customer-facing vs internal）如真实需要，用独立属性表达（候选，未启用），
  不动 error.code 文法。
- SCOPE 段不做强制注册校验：命名空间是文档约定 + Error Registry 全码注册（ADR-0015），
  未注册码保持宽松。

## 补充决策（2026-08-13）

同日讨论进一步明确三条约束，作为上述决策的细化：

- **SCOPE 归属由失败面（failure surface）决定**，不由包装它的业务模块决定：业务/校验拒绝
  （KindBusiness / KindValidation）用业务模块 SCOPE（ORDER/PAYMENT/USER…）；基础设施故障
  （KindSystem，失败面为 DB/缓存/MQ/网络等依赖组件）用 `INFRA.*`（OPERATION=组件）；其余
  系统类（非基础设施，如并发冲突 ABORTED）可保留领域 SCOPE 或保持无码。黑盒据此把
  `USER.ROLE_UPDATE.DB_TIMEOUT` 迁移为 `INFRA.MYSQL.QUERY_TIMEOUT`、
  `MESSAGING.PUBLISH.DEADLINE_EXCEEDED` 迁移为 `INFRA.MQ.PUBLISH_TIMEOUT`；
  `LOCK.ACQUIRE.CONFLICT`（并发冲突，非基础设施）保留领域 SCOPE。
- **INTERNAL/panic 不承载 error.code**：panic 是程序缺陷，既无业务码也无 INFRA 失败面；
  定位靠 error.type=INTERNAL + event.name（runtime.panic.occurred）+ stacktrace + code.*，
  不设具体错误码。
- **event.name 与 error.code 前两段不做 MUST 对齐**：业务码 SHOULD 与 event.name 的
  domain.subject 同源（如 user.role_update ↔ USER.ROLE_UPDATE，第三段不必一致）；
  INFRA.* 与业务事件名天然不同源（失败面 vs 业务事实），且同一失败码可复用于多个业务事件
  （如 INFRA.MYSQL.QUERY_TIMEOUT 同时出现在 user.role_update.database_timeout 与
  order.create.database_timeout），这是「跨操作聚合同一基础设施故障」的价值。
  因此「event.name 与 error.code 前两段必须相同」作为备选方案被否掉。

## 结果（Consequences）

- 四字段体系只有一套 `SCOPE.OPERATION.REASON` 文法要记，不会因「业务/系统」维度长出第二套解析规则。
- `INFRA.*` 前缀可统一检索基础设施故障，再配 error.type（UNAVAILABLE / DEADLINE_EXCEEDED…）细分。
- 黑盒示例新增 `INFRA.REDIS.UNAVAILABLE`（error.type=UNAVAILABLE、app.upstream_service=redis），
  验证系统错误码直接复用业务码文法。
- 依赖名不占 error.code 段位，技术栈替换不产生错误码迁移。
