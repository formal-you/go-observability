# example/mall — 接入方业务事件注册表（C2 / B4）

核心库 **不** 内置电商 `business.*` 常量。本包示范：

1. 自建 `EventName` 常量注册表（10 个 B4 事件）
2. 自建领域 `app.*` 专属键
3. 经 `BusinessPayload.ExtraAttrs` 注入

商城系统可直接复制本模式到自己的 `internal/observability` 包。

```go
logger.Emit(ctx, log.BusinessEvent{
    EventMetadata: log.EventMetadata{Level: log.LevelInfo, TraceID: tid, SpanID: sid},
    Data: log.BusinessPayload{
        EventName:  mall.EventOrderPaid,
        Result:     log.ResultSuccess,
        ExtraAttrs: mall.OrderPaidExtra(orderID, "wechat", paidAt, amountCents),
    },
})
```
