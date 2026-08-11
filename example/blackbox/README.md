# example/blackbox

真实日志黑盒样例：file 模式由 `telemetry.NewFileRuntime` 在不连接 Collector 的情况下生成有效 span，通过 Gin 中间件执行四个 HTTP 请求，另生成两个后台错误场景。测试同时锁定带服务身份的 JSONL 运营投影与 OTLP LogRecord 语义。

## 本地 JSONL

```powershell
go run ./example/blackbox -config "example/blackbox/config.example.yaml"
go test ./example/blackbox -v
```

程序读取 [`config.example.yaml`](config.example.yaml)，写入配置指定的 JSONL 路径，并打印各 request_id / 后台场景对应的 trace_id。配置属于 blackbox 应用层，使用严格 YAML 解析后映射到 `telemetry.Config`、`log.WithMinLevel` 和 `file.WithRotation`；核心库不会自动寻找配置文件。

| 配置 | 作用 |
| --- | --- |
| `service.*` | 服务名、版本、实例和环境，注入每条 file 日志 |
| `logs.level` | 最低写出级别：`DEBUG` / `INFO` / `WARN` / `ERROR` |
| `logs.output_path` | JSONL 输出路径 |
| `logs.overwrite_on_start` | 启动时清理当前文件，保证黑盒结果不累计 |
| `logs.rotation.max_size_mb` | 单个日志文件的最大大小 |
| `logs.rotation.max_backups` | 最多保留的轮转备份数量，`0` 表示不按数量清理 |
| `logs.rotation.max_age_days` | 备份保留天数，`0` 表示不按天数清理 |
| `logs.rotation.compress` | 是否 gzip 压缩旧日志 |
| `otlp.endpoint` | OTLP 模式的 Collector gRPC 地址，可被 `-endpoint` 覆盖 |

`overwrite_on_start=true` 是为了让黑盒测试可重复验收；常驻生产进程通常设为 `false`，由轮转策略控制文件大小和保留周期。

## 场景与事件

| 场景 | 状态 | 必须记录的事件 |
| --- | --- | --- |
| 订单支付成功 | 200 | order.payment.succeeded + http.request.completed |
| 库存不足 | 409 | BusinessEvent（http.request.rejected）+ http.request.completed |
| 高风险输入触发数据库故障 | 500 | ErrorEvent + SecurityEvent + AuditEvent + AccessEvent |
| panic | 500 | runtime.panic.occurred + http.request.completed |
| 后台 MQ 发布失败 | 无 HTTP | messaging.publish.failed，不伪造 AccessEvent/request_id |
| 后台锁冲突 | 无 HTTP | lock.acquire.failed，不伪造 AccessEvent/request_id |

同一 HTTP 请求的事件共享 trace_id、span_id、request_id；HTTP method/path/status/latency 只在 AccessEvent 中出现。默认 Logger 不配置 Sampler，因此 business success 也始终有对应 AccessEvent。健康检查应使用 `AccessConfig.SkipPaths` 排除。

## 本地 LGTM 联调

Collector、Tempo、Loki、Grafana 已启动时运行：

```powershell
go run ./example/blackbox -mode otlp -config "example/blackbox/config.example.yaml"
```

OTLP 模式固定 `TraceSampleRatio=1`、`service.name=go-observability-blackbox`，退出前关闭 Provider 以刷新批次。Loki/Grafana 查询：

```logql
{service_name="go-observability-blackbox"} | json
```

从日志复制 trace_id 后在 Tempo 查询，即可核对 LogRecord 与 Trace 的关联。

## 字段顺序

file/stdout 固定为 `timestamp -> level -> msg -> service metadata -> trace/span/request/latency -> event.name -> payload -> app.result`。黑盒测试会锁定 timestamp 首列及关键字段相对位置。
