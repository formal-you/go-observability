# example/blackbox

真实日志黑盒样例：由 OTel SDK 生成有效 span，通过 Gin 中间件执行四个 HTTP 请求，另生成两个后台错误场景。测试同时锁定 JSONL 运营投影与 OTLP LogRecord 语义，不依赖外部 Collector。

## 本地 JSONL

```powershell
go run ./example/blackbox
go test ./example/blackbox -v
```

程序覆盖写入 `example/blackbox/sample.jsonl`（每行一个 JSON 对象，`*.jsonl` 已忽略），并打印各 request_id / 后台场景对应的 trace_id。

## 场景与事件

| 场景 | 状态 | 必须记录的事件 |
| --- | --- | --- |
| 订单支付成功 | 200 | business.order.paid + access.http.request |
| 库存不足 | 409 | BusinessEvent（error.http.request）+ access.http.request |
| 高风险输入触发数据库故障 | 500 | ErrorEvent + SecurityEvent + AuditEvent + AccessEvent |
| panic | 500 | error.runtime.panic + access.http.request |
| 后台 MQ 发布失败 | 无 HTTP | error.mq.publish，不伪造 AccessEvent/request_id |
| 后台锁冲突 | 无 HTTP | error.lock.conflict，不伪造 AccessEvent/request_id |

同一 HTTP 请求的事件共享 trace_id、span_id、request_id；HTTP method/path/status/latency 只在 AccessEvent 中出现。默认 Logger 不配置 Sampler，因此 business success 也始终有对应 AccessEvent。健康检查应使用 `AccessConfig.SkipPaths` 排除。

## 本地 LGTM 联调

Collector、Tempo、Loki、Grafana 已启动时运行：

```powershell
go run ./example/blackbox -mode otlp -endpoint "127.0.0.1:4317"
```

OTLP 模式固定 `TraceSampleRatio=1`、`service.name=go-observability-blackbox`，退出前关闭 Provider 以刷新批次。Loki/Grafana 查询：

```logql
{service_name="go-observability-blackbox"} | json
```

从日志复制 trace_id 后在 Tempo 查询，即可核对 LogRecord 与 Trace 的关联。

## 字段顺序

file/stdout 固定为 `timestamp? -> level -> msg -> trace/span/request/latency -> event.name -> payload -> app.result`。测试按相对位置断言，timestamp 省略或存在都不会误报。
