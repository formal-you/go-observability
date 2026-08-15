# 01 · 六类类型化事件：让日志先有“类型”

## 一、为什么自由字符串不够

`slog.Info("order paid", "order_id", id)` 看似清晰，但下游很难统一处理：

- 告警系统不知道哪些日志是 `error`、哪些是 `access`；
- 检索系统只能做字符串匹配，不能按 `type` 或 `event.name` 聚合；
- 每换一个团队，字段名和事件名都不一样。

`go-observability` 把事件先分成六个稳定粗分类，再用 `event.name` 表达具体事实。

## 二、六类事件

| `type` | 载荷 | 典型场景 |
| --- | --- | --- |
| `access` | `AccessPayload` | 每请求一条，HTTP/RPC 访问与延迟 |
| `business` | `BusinessPayload` | 业务动作、业务拒绝 |
| `error` | `ErrorPayload` | 系统错误、panic、依赖失败 |
| `security` | `SecurityPayload` | 认证、鉴权、风控拦截 |
| `audit` | `AuditPayload` | 高权限操作与敏感资源变更 |
| `probe` | `ProbePayload` | 健康、就绪、存活探测 |

分类不是装饰：`type` 是稳定短名，`event.name` 是 `<domain>.<subject>.<event>` 的具体事实名。

## 三、动手

从仓库根目录运行：

```powershell
go run ./example/02_events
Get-Content .\logs\events-six.jsonl
```

```bash
go run ./example/02_events
cat ./logs/events-six.jsonl
```

`02_events` 会写出六条事件，固定了时间和链路标识，方便课堂对照。

## 四、输出解读

以 `business` 事件为例：

```json
{"timestamp":"2026-08-15T12:00:00Z","level":"INFO","type":"business","service.name":"events-demo","trace_id":"0123456789abcdef0123456789abcdef","span_id":"0123456789abcdef","request_id":"req-002","event.name":"order.payment.succeeded","user.id":"u_1001","app.resource_type":"order","app.resource_id":"ORD-1001","app.order_id":"ORD-1001","app.amount_cents":9900,"app.pay_channel":"wechat","app.result":"success"}
```

字段来源分三类：

- `service.name` 来自 OTel Resource，进程级身份；
- `trace_id` / `span_id` / `request_id` 来自 `EventMetadata`，请求级关联；
- `event.name`、`app.*` 来自 `BusinessPayload`，业务语义。

领域字段只能通过 `ExtraAttrs` 注入 `app.*` 键，核心库不会散落登记具体业务字段。


## 代码对比：一条访问日志

传统写法：

```go
slog.Info("request completed",
    "method", r.Method,
    "path", r.URL.Path,
    "status", 200,
    "latency_ms", 12,
)
```

`go-observability`：

```go
logger.Emit(ctx, log.AccessEvent{
    EventMetadata: log.EventMetadata{
        Level:     log.LevelInfo,
        LatencyMS: 12,
    },
    Data: log.AccessPayload{
        EventName: log.EventNameHTTPRequestCompleted,
        HTTP: log.HTTPInfo{
            Method:     r.Method,
            URLPath:    r.URL.Path,
            StatusCode: 200,
        },
        Result: log.ResultSuccess,
    },
})
```

前者是自由字段，后者把 HTTP 语义固定到 `HTTPInfo`，跨服务查询统一。

## 五、最佳实践

1. 事件名三段式：`<domain>.<subject>.<event>`，例如 `order.payment.succeeded`；
2. 首段不要重复 `type` 粗分类，否则就是冗余；
3. 领域键统一 `app.*` 前缀，避免与 OTel Semconv 冲突；
4. 健康检查用 `probe` 事件，和访问日志分开，便于确定性排除噪音。

下一篇：[02 · 错误建模与投影](02-error-model.md)。
