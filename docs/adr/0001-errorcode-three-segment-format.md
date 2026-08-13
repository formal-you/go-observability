# ADR-0001：ErrorCode 采用三段式（服务/模块）.（场景/操作）.（结果/具体错误）

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0016（ErrorType OTel/gRPC 标准枚举，取代 ADR-0002）

## 背景（Context）

业务错误码用于在纷繁复杂的业务场景中提供精确定位，回答「具体在哪里的什么业务逻辑出了什么问题」，
面向人与诊断：Trace / Log / API 响应 / 前端提示。此类码基数（Cardinality）天然很高，
系统中可能有成百上千个，属于高基数标签。

原注释写作「模块.场景.操作」与真实语义不符：第一段实际是服务/模块边界，第三段是结果/具体错误，
且「模块」一词容易与 ErrorType 的资源域（domain）概念混用。

## 决策（Decision）

ErrorCode 采用三段式：

    （服务/模块）.（场景/操作）.（结果/具体错误）

示例：`ORDER.CREATE.STOCK_INSUFFICIENT`
（服务/模块=ORDER，场景/操作=CREATE，结果/具体错误=STOCK_INSUFFICIENT）。

- 仅 `BizError` 承载 ErrorCode；`SystemError` 可通过 `WithCode` 关联可选业务码（未设置时为空）。
- ErrorCode 是**高基数**标签：只用于 Trace / Log / API / 前端提示，**禁止**作为 Metrics 维度。
- 定义时与 ErrorType 显式绑定：`NewBusiness(code, typ, message)` 同时传入具体码与低基数类别。

## 结果（Consequences）

- 正面：精确定位、人可读、可跨服务复现同一业务错误。
- 代价：高基数，不能进监控维度；调用方需要为每个业务错误维护一个码（建议配注册表/常量）。
- 边界：EventName 同样采用三段式（`<domain>.<subject>.<event>`，见 ADR-0018 / `EventNamePattern`），
  但两者分层不同、不混用；EventName 是事件名，ErrorCode 是业务错误码。
