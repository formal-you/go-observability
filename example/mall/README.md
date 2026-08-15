# 领域业务事件示例

核心库不内置电商 `business.*` 常量。本包演示在使用方代码中维护 Event Registry（事件名注册表）、领域 `app.*` 属性，并通过 `BusinessPayload.ExtraAttrs` 输出。

```go
logger.Emit(ctx, log.BusinessEvent{
	EventMetadata: log.EventMetadata{Level: log.LevelInfo},
	Data: log.BusinessPayload{
		EventName:  mall.EventOrderPaid,
		Result:     log.ResultSuccess,
		ExtraAttrs: mall.OrderPaidExtra(orderID, "wechat", paidAt, amountCents),
	},
})
```

从仓库根目录验证注册表：

```bash
go test ./example/mall
```

接入方应把同类注册表放在自己的领域或 observability 包中，并用测试固定事件名和字段契约。
