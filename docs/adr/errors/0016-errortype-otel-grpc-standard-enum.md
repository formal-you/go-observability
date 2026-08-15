# ADR 0016：ErrorType 复用 OTel/gRPC 标准枚举（跨模块）

  状态：Accepted
  日期：2026 08 13
  关联：取代 ADR 0002（domain.reason）；关联 ADR 0001、ADR 0010、ADR 0015

## 背景（Context）

原 ErrorType 采用 `domain.reason` 自定义词表（db.connection_error / business.stock_insufficient 等），
第一段是资源域，属于模块内分类：不同服务各自维护一份枚举，聚合/告警规则随词表漂移，维护成本高，
且与 OTel/gRPC 生态不直接兼容。改造方案要求：error.type 是跨模块的标准分类，复用 OTel/gRPC
标准枚举（以通用性换取工具链兼容与低维护成本）。

## 为什么 error.type 采用 gRPC canonical code

- 低基数：可安全作为 metrics 维度与告警路由标签。
- 跨服务闭合枚举：不同服务不再各自维护 domain.reason 词表，聚合规则不漂移。
- 生态原生：OTel / gRPC / Grafana 能直接识别，无需维护自定义枚举映射。

具体业务错误仍由 `error.code` 表达；`error.type` 只负责“失败属于哪一类”。

## 决策（Decision）

  ErrorType 映射 OTel error.type，复用 **OTel/gRPC 标准枚举（gRPC canonical code）**：跨模块、
  闭合、单段 UPPER_SNAKE，共 16 个取值（UNKNOWN / CANCELLED / INVALID_ARGUMENT /
  DEADLINE_EXCEEDED / NOT_FOUND / ALREADY_EXISTS / PERMISSION_DENIED / RESOURCE_EXHAUSTED /
  FAILED_PRECONDITION / ABORTED / OUT_OF_RANGE / UNIMPLEMENTED / INTERNAL / UNAVAILABLE /
  DATA_LOSS / UNAUTHENTICATED）。
  `Validate` 校验枚举成员，禁止自定义词表。
  业务/校验预期拒绝使用 4xx 类客户端码：INVALID_ARGUMENT / FAILED_PRECONDITION / ALREADY_EXISTS /
  NOT_FOUND / PERMISSION_DENIED / UNAUTHENTICATED / OUT_OF_RANGE / RESOURCE_EXHAUSTED；
  非预期系统故障使用其余码：UNKNOWN / CANCELLED / DEADLINE_EXCEEDED / ABORTED / UNIMPLEMENTED /
  INTERNAL / UNAVAILABLE / DATA_LOSS。
  严格构造器按上述集合约束：`NewBusinessError` 要求业务集合；`NewSystemError` 拒绝业务集合。
  堆栈策略按 code 精确覆盖：UNKNOWN / INTERNAL / UNAVAILABLE / DEADLINE_EXCEEDED / DATA_LOSS →
  must；CANCELLED / ABORTED → optional；其余（含业务预期拒绝、UNIMPLEMENTED、未知值）→ none。
  `INTERNAL`（含 panic）不得被降为 none。
  旧 domain.reason 常量移除；迁移映射：db.query_timeout → DEADLINE_EXCEEDED、
  db.connection_error → UNAVAILABLE、runtime.panic → INTERNAL、business.* → FAILED_PRECONDITION 等。

## 结果（Consequences）

  正面：跨服务统一错误词表，Grafana / OTel / gRPC 工具链原生识别，无需维护自定义枚举。
  代价：丢失资源域细分（db/redis/mq 归入 UNAVAILABLE / DEADLINE_EXCEEDED 等）；具体业务范围
  由 ErrorCode、EventName 或事件属性表达。
  兼容性：破坏性 schema 变更（error.type 值 + 常量名），CHANGELOG 给出迁移映射；v0.x 允许。
