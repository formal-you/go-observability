# ADR-0010：严格错误构造与 SCOPE.OPERATION.REASON / ErrorType 标准枚举

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0001、ADR-0009、ADR-0016（取代 ADR-0002 的 ErrorType 部分）
- 注：ErrorType 文法与命名空间部分已被 [ADR-0016](0016-errortype-otel-grpc-standard-enum.md) 取代；ErrorCode 的 SCOPE.OPERATION.REASON 部分保持有效。

## 背景（Context）

`ErrorCode` 与 `ErrorType` 已有语义约定，但公开类型仍可由任意字符串强制转换，旧位置参数构造器也无法在构造期返回格式错误。非法值因此可能直到日志查询或指标聚合时才暴露。

## 决策（Decision）

- ErrorCode 的规范文法为 `SCOPE.OPERATION.REASON`：恰好三段，每段只允许大写字母、数字和下划线。SCOPE 表示服务或业务模块，不称为 domain。
- `errs.ErrorCodePattern` 是 SCOPE.OPERATION.REASON 的规范正则，`Validate` 使用正则校验（等价文法）。
- ErrorType 复用 OTel/gRPC 标准枚举（gRPC canonical code，单段 UPPER_SNAKE，闭合枚举），文法见 [ADR-0016](0016-errortype-otel-grpc-standard-enum.md)；`ParseErrorType` 校验枚举成员。
- `Validate` 校验已有值；`ParseErrorCode` / `ParseErrorType` 返回规范类型或错误，不做裁剪、大小写转换或自动修复。
- `NewValidationError`、`NewBusinessError`、`NewSystemError` 使用各自配置结构并返回 `(错误值, error)`。严格构造器校验消息、命名空间和重试状态，并把 cause 写入 `Unwrap` 链。
- BusinessError 必须使用业务预期拒绝集合（INVALID_ARGUMENT / FAILED_PRECONDITION / ALREADY_EXISTS / NOT_FOUND / PERMISSION_DENIED / UNAUTHENTICATED / OUT_OF_RANGE / RESOURCE_EXHAUSTED）；SystemError 拒绝该集合（见 ADR-0016）。多个 ErrorCode 可以归入同一个 ErrorType，库不自动推导二者（注册映射见 ADR-0015）。
- 旧构造器与 SystemOption 作为 Deprecated 兼容层保留，维持原运行行为，不追加 panic 校验。

## 结果（Consequences）

- 新代码在错误构造边界拒绝语义漂移，调用方必须处理构造错误。
- 旧 `domain.reason` ErrorType 迁移为 OTel/gRPC 标准枚举（如 `db.query_timeout → DEADLINE_EXCEEDED`，见 ADR-0016）；更具体的业务范围由 error.code、event.name 或事件属性表达。
- 旧调用方可以渐进迁移，但新示例与正式黑盒只使用严格构造器。
