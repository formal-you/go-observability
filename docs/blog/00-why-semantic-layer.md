# 00 · 为什么你的日志还需要一个“语义层”

## 一、先说痛点

很多 Go 项目已经用了 `slog`、`zap` 或 `zerolog`，日志本身是结构化的，但排查问题时仍然很痛：

- **字段漂移**：每个模块自己发明 key，查问题先猜字段名；
- **链路断裂**：Log、Trace、Metric 各自初始化，故障现场拼不起来；
- **错误失真**：`error` 经 `%w` 包装后，retry、source、stack 悄悄丢失；
- **脱敏有漏网**：token 藏在 map、slice、group 或 `LogValuer` 里继续外泄；
- **出口形状混乱**：file、stdout、OTLP 共用“万能结构”，两边都不好用。

这些痛点的共同原因不是“没打日志”，而是**没有一个稳定的语义契约**。

## 二、本项目定位

`go-observability` 是 `slog` 与 OpenTelemetry 之间的语义层：

```text
slog         -> 结构化字段
OpenTelemetry -> 传输协议
go-observability -> 稳定的语义契约：六类事件、字段来源、错误投影、治理策略
```

它不替代 `slog` 或 OTel，而是把团队最容易失控的约定固化成可测试的 Go API。

## 三、30 秒跑通

从仓库根目录运行：

```powershell
go run ./example/01_quickstart
Get-Content .\logs\events.jsonl
```

```bash
go run ./example/01_quickstart
tail -n 1 ./logs/events.jsonl
```

你会得到一条可检索的 JSONL：

```json
{"timestamp":"2026-08-15T12:00:00Z","level":"INFO","type":"business","service.name":"mall-monolith","trace_id":"...","span_id":"...","event.name":"order.payment.succeeded","app.result":"success"}
```

这条日志的关键不是“它是 JSON”，而是：

- `type=business`：粗分类稳定；
- `event.name=order.payment.succeeded`：事实名稳定；
- `trace_id` / `span_id`：与 Trace 关联；
- `app.result=success`：业务结果稳定，可用于采样与告警。

## 四、和常见库的差异

| 能力 | `slog` / `zap` | `go-observability` 增量价值 |
| --- | --- | --- |
| 结构化输出 | ✅ | ✅ |
| 类型化事件模型 | ❌ 自由字符串 | ✅ access/business/error/security/audit/probe |
| 字段源规范 | ❌ 团队自觉 | ✅ OTel Semconv + `app.*` |
| JSONL 与 OTLP 双投影 | ❌ 需自己实现 | ✅ 同一事件两种投影 |
| 错误投影 | ❌ 手动 | ✅ `errs.AppError -> EventFromError` |
| 采样/脱敏治理 | ❌ 分散 | ✅ Sampler / Masker 注入点 |
| 链路关联 | ⚠️ 手动 | ✅ TraceExtractor 自动补全 |


## 代码对比：同样记录一次支付成功

传统 `slog` 写法：

```go
slog.Info("order paid",
    "order_id", orderID,
    "amount_cents", amountCents,
    "channel", "wechat",
)
```

`go-observability` 写法：

```go
logger.Emit(ctx, log.BusinessEvent{
    EventMetadata: log.EventMetadata{Level: log.LevelInfo},
    Data: log.BusinessPayload{
        EventName: log.NewEventName("order", "payment", "succeeded"),
        Resource:  log.Resource{Type: "order", ID: orderID},
        Result:    log.ResultSuccess,
        ExtraAttrs: []slog.Attr{
            slog.String("app.order_id", orderID),
            slog.Int64("app.amount_cents", amountCents),
            slog.String("app.pay_channel", "wechat"),
        },
    },
})
```

后者把 `event.name`、`app.result`、`app.*` 都固定下来，查询和告警不再靠猜字段名。

## 五、这套系列会讲什么

| 文章 | 主问题 |
| --- | --- |
| 01 | 六类事件如何被查询、告警和治理 |
| 02 | 错误如何建模、如何投影成事件 |
| 03 | 采样、脱敏、多出口如何落地 |
| 04 | Trace/Metric/Log 如何统一装配 |
| 05 | Gin/http/gRPC/kratos 如何接入 |
| 06 | Security/Audit 如何自动留痕 |
| 07 | JSONL 与 OTLP 如何双投影 |
| 08 | 用一个 mall 案例串起全部能力 |

下一篇：[01 · 六类类型化事件](01-typed-events.md)。
