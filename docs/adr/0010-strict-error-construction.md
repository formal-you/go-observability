# ADR-0010：严格错误构造与 SCOPE.OPERATION.REASON / domain.reason

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0001、ADR-0002、ADR-0009

## 背景（Context）

`ErrorCode` 与 `ErrorType` 已有语义约定，但公开类型仍可由任意字符串强制转换，旧位置参数构造器也无法在构造期返回格式错误。非法值因此可能直到日志查询或指标聚合时才暴露。

## 决策（Decision）

- ErrorCode 的规范文法为 `SCOPE.OPERATION.REASON`：恰好三段，每段只允许大写字母、数字和下划线。SCOPE 表示服务或业务模块，不称为 domain。
- ErrorType 的规范文法为 `domain.reason`：恰好两段，每段只允许小写字母、数字和下划线，并保持低基数。
- `Validate` 校验已有值；`ParseErrorCode` / `ParseErrorType` 返回规范类型或错误，不做裁剪、大小写转换或自动修复。
- `NewValidationError`、`NewBusinessError`、`NewSystemError` 使用各自配置结构并返回 `(错误值, error)`。严格构造器校验消息、命名空间和重试状态，并把 cause 写入 `Unwrap` 链。
- BusinessError 必须使用 `business.*`；SystemError 拒绝 `business.*` 与 `validation.*`。多个 ErrorCode 可以归入同一个 ErrorType，库不自动推导二者。
- 旧构造器与 SystemOption 作为 Deprecated 兼容层保留，维持原运行行为，不追加 panic 校验。

## 结果（Consequences）

- 新代码在错误构造边界拒绝语义漂移，调用方必须处理构造错误。
- `business.order.stock_insufficient` 等三段 ErrorType 迁移为 `business.stock_insufficient`；更具体的业务范围由 ErrorCode、EventName 或事件属性表达。
- 旧调用方可以渐进迁移，但新示例与正式黑盒只使用严格构造器。
