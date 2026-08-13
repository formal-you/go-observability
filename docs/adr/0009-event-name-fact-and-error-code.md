# ADR-0009：EventName 使用事实名，ErrorCode 独立投影

- 状态：Accepted
- 日期：2026-08-11

## 背景（Context）

`type`（Writer 首个参数）已承载 access/business/error/security/audit/probe 粗分类。旧 EventName 再以同一类别开头会产生可推导冗余，也会出现 `type=business`、`event.name=error.http.request` 的矛盾。与此同时，BizError 的具体错误码写入 `app.business_code`，SystemError 的可选错误码却借用 `app.operation`，相同概念形成两个查询键。

## 决策（Decision）

- EventName 保留严格三段式，语义改为「领域.对象.事实」，例如 `order.payment.succeeded`、`order.create.stock_insufficient`、`runtime.panic.occurred`。
- `access` / `business` / `error` / `security` / `audit` / `probe` 只由 `type` / EventType 表达，禁止作为 EventName 首段；事件身份由 `(type, event.name)` 共同确定。
- `error.type` 表达低基数失败类别，`error.code` 表达可选的稳定具体错误码；两者都不用于自动生成 EventName。
- BizError 与 SystemError 的 `ErrCode()` 统一投影到 `error.code`。HTTP 错误出口不再提供泛化错误事件名（见 ADR-0018），接入方必须经 `EventNameResolver` 提供符合 `EventNamePattern` 的具体事实名。

## 结果（Consequences）

- 查询不再依赖 EventName 重复携带粗分类；需要完整事件身份时同时筛选 `type` 与 `event.name`。
- “业务全量 + access 采样”应使用 EventType 感知的采样器；按 `event.name` 的采样只表达领域前缀。旧类别前缀配置由 Logger 兼容识别。
- `app.business_code` 与 `app.operation` 不再写出。旧 Go 字段和常量只作为源码兼容别名，输出始终使用新语义。
- 这是 schema 迁移；文档、示例、黑盒测试和查询规则必须同步更新。
