# ADR-0015：Error Registry——error.code 到 error.type 的固定映射

- 状态：Accepted
- 日期：2026-08-12
- 关联：ADR-0001（ErrorCode 三段式）、ADR-0016（ErrorType OTel/gRPC 标准枚举）、ADR-0010（严格错误构造）

## 背景（Context）

ErrorType 与 ErrorCode 是「多对一」：多个三段式 ErrorCode 归入同一个低基数
ErrorType。但该关系只停留在文档层面：错误对象独立携带 code 与 type，任何 code 都可以配任意
type，既无法在构造期发现「同一 code 映射到不同 type」的漂移，也无法回答「这个 error.code
到底归属哪个 error.type」。改造方案要求语义注册表化：`error.code → exactly one error.type`。

## 决策（Decision）

- 新增 Error Registry（errs 包内）：`RegisterErrorCode(code, typ)` 把 ErrorCode 注册到恰好一个
  ErrorType；同一 code 重复注册必须类型一致（幂等成功），不一致返回错误。
- `ErrorCode.RegisteredErrorType()` 反查注册映射；未注册返回 false。
- 严格构造器（`NewBusinessError` / `NewSystemError`）在校验 code/type 文法后，对**已注册**的
  code 强制 type 一致性：不一致则构造失败。未注册的 code 保持既有行为（不强制映射），注册是
  使用方显式选择，核心库不预知领域码。
- 注册应在进程启动期、构造错误前完成（与 `SetStackPolicy` 同类）；Registry 并发安全。
- 旧构造器（`NewBusiness` / `NewSystem` + `WithCode`）保持宽松，不做映射校验（Deprecated 兼容路径）。

## 结果（Consequences）

- 正面：code→type 漂移在「启动期注册 + 构造期校验」双保险下被尽早发现；查询/告警可依赖
  code 反查 type（多对一聚合）。
- 代价：使用方需要为受治理的业务码显式注册（建议维护注册表/常量，与 Event Registry 同层）。
- 边界：这是 schema/治理强化，不是自动注册；未注册码不改变现有输出与行为。

## 补充决策（2026-08-13）

- Error Registry 扩展为 `ErrorCode → {ErrorType, EventName?}`：新增
  `RegisterErrorContract(code, typ, eventName)` 一次性注册「错误码 + type + 错误事件名」
  （系统/基础设施错误码路径）与 `ErrorCode.RegisteredEventName()` 反查；`RegisterErrorCode`
  保留为纯 code → type 注册（事件名留空，业务码路径）。
- EventName 在 errs 内为不透明字符串：errs 只依赖标准库（不 import log，避免与 log 包
  循环依赖），文法（`log.EventNamePattern`）由接入方适配层校验后传入（如 mall
  `errorx.RegisterSystemError`）。
- 事件名不是必填：业务/校验错误不注册事件名（其领域事件由 Application 发布）；空事件名
  可在后续以「code-only 升级为完整契约」补齐（type 必须一致）。
- 重复注册规则：type 必须一致；事件名已存在时必须一致（幂等成功），不一致返回错误——
  error.code → exactly one error.type / one event name 是查询、告警与事件写出的依据。
- 新增 Must 变体 `MustRegisterErrorCode` / `MustRegisterErrorContract`：注册失败
  （非法文法/重复冲突）直接 panic；注册输入通常是常量/全局预定义，失败即编码错误，
  应在启动期尽早暴露（与 `log.NewEventName` 同类）。error 变体保留给需要处理失败的
  调用方（如测试断言拒绝路径）。