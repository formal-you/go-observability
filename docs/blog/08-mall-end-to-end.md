# 08 · mall 端到端诊断故事

## 一、目标

前面 0-7 篇分别讲了能力，这一篇用一个可运行的 `mall` 服务把它们串起来，回答：

> 一次业务失败，如何从日志跳到 trace，再从 trace 跳到 metric，最终定位问题？

## 二、启动 mall

从仓库根目录运行：

```powershell
go run ./example/mall/cmd
```

```bash
go run ./example/mall/cmd
```

默认监听 `:8083`，写 `logs/mall.jsonl`。

## 三、制造一次业务失败

```powershell
Invoke-WebRequest -Method Post http://127.0.0.1:8083/api/v1/orders -Headers @{ 'X-Fail' = 'stock' }
Get-Content .\logs\mall.jsonl
```

```bash
curl -X POST -H 'X-Fail: stock' http://127.0.0.1:8083/api/v1/orders
tail -n 1 ./logs/mall.jsonl
```

你会看到：

- HTTP 返回 409；
- JSONL 中出现 `type=business` 的 `order.create.stock_insufficient` 错误事件；
- 同时还有 `http.request.completed` access 事件。

## 四、关联关系

同一请求的事件共享：

```text
trace_id -> 跨服务关联主键
span_id  -> 单服务内定位到具体子 span
request_id -> 对外报障凭证
```

在 JSONL 里找到 `trace_id`，就能去 Tempo 查看对应 span；查看 `http.server.request.duration` 指标，能看到这个路由的耗时分布。

## 五、mall 里展示了什么

| 能力 | 对应实现 |
| --- | --- |
| 事件注册表 | `example/mall/events.go` |
| 错误建模 | `ORDER.CREATE.STOCK_INSUFFICIENT` |
| 治理 | `FieldMasker` + `EventTypeKeepSampler` |
| 全链路中间件 | `Trace/Access/Recover/Metrics/Security/Audit/ErrorResponse` |
| 安全与审计 | `SecurityLog` / `AuditLog` |
| 三信号装配 | `telemetry.NewRuntime` |

## 六、黑盒验收

仓库还提供 `example/blackbox`，用真实 Gin 请求和 OTel span 锁定 JSONL 与 OTLP 语义：

```powershell
go run ./example/blackbox -config "example/blackbox/config.example.yaml"
go test ./example/blackbox -v
```

它保证示例承诺的行为是可复现的，而不是文档写写而已。


## 代码对比：端到端之前 vs 之后

传统 mall 可能把日志、错误、安全和链路分散在各 handler；本示例把注册表、错误体系、治理和中间件组合进一个服务：

```go
logger := log.NewLogger(w,
    log.WithTraceExtractor(otelutil.NewTraceExtractor()),
    log.WithMasker(log.FieldMasker{}),
    log.WithSampler(log.EventTypeKeepSampler{
        KeepTypes: []log.EventType{log.EventBusiness, log.EventError, log.EventSecurity, log.EventAudit},
        Fallback:  log.NewResultKeepSampler(1.0),
    }),
)
```

配合 `mall.EventOrderCreated` / `EventProductViewed` 注册表，业务代码只构造领域事实，不再手写字段名。

## 七、下一步

把本系列文章配合仓库示例一起读：先跑 `01_quickstart`，再按编号课程推进；遇到问题从 `blackbox` 反查字段契约。

欢迎把具体场景或框架接入需求提成 Issue，让示例体系继续长出来。
