# example/blackbox

黑盒日志样例：生成覆盖 **access / business / error** 三类事件的 JSON 日志，供人工审核，
并配套结构断言测试防止实现漂移。

## 运行

```powershell
go run ./example/blackbox
```

输出 `example/blackbox/sample.jsonl`（每行一个 JSON 对象；`*.jsonl` 已被 .gitignore 忽略，
每次运行重新生成）。结构校验由测试完成：

```powershell
go test ./example/blackbox -v
```

## 覆盖的事件（9 条，顺序固定）

| # | 类别 | 事件 | 关键结构 |
| --- | --- | --- | --- |
| 0 | access | HTTP 200 | msg=access、level=INFO、app.result=success |
| 1 | access | HTTP 404 | msg=access、level=WARN、app.result=failed |
| 2 | business | 订单支付成功 | msg=business、event.name=business.order.paid、app.result=success |
| 3 | business | 库存不足（业务拒绝） | app.business_code=ORDER.CREATE.STOCK_INSUFFICIENT、app.result=blocked |
| 4 | business | 参数校验失败 | error.type=validation.failed、app.result=failed（errs 投影） |
| 5 | error | DB 连接失败（非预期系统错误） | **exception.stacktrace 有堆栈**、app.retryable=true、level=WARN |
| 6 | error | MQ 发布失败（重试耗尽） | **exception.stacktrace 有堆栈**、level=ERROR、app.retry_count=3 |
| 7 | error | 运行时 panic | **exception.stacktrace 有堆栈**、event.name=error.runtime.panic、level=ERROR |
| 8 | error | 锁冲突（对比项） | **exception.stacktrace 无堆栈**（StackOptional）、level=ERROR |

## 字段顺序（所有事件一致）

每条日志的字段按固定顺序输出，跨事件保持一致，便于人眼扫描与工具解析
（由 `writer/file` 保证，测试锁定）：

| 顺序 | 字段 | 说明 |
| --- | --- | --- |
| 1 | `timestamp` | 事件时间（未设置时省略） |
| 2 | `level` | 语义化级别 |
| 3 | `msg` | 事件粗分类（access/business/error…） |
| 4 | `trace_id` / `span_id` / `request_id` / `latency_ms` | 链路与延迟（未设置时省略） |
| 5 | `event.name` | 三段式细名 |
| 6 | 其余 payload 字段 | 按事件构造顺序（http.*、error.type、exception.*、app.*…） |
| 7 | `app.result` | 恒为最后，一眼看结局 |

## 非预期系统错误的结构（第 5/6/7 条）

`errs.NewSystem` 使用 StackMust 类别（db./redis./mq./http./runtime.）时，构造点自动采集
创建点堆栈，经 `EventFromError` 投影为 ErrorEvent，JSON 关键字段：

| JSON 键 | 含义 |
| --- | --- |
| msg | 粗分类 event_type（=error） |
| event.name | 三段式细名（如 error.db.connection） |
| error.type | 低基数失败类别（如 db.connection_error） |
| exception.message | 错误消息 |
| exception.stacktrace | **构造点堆栈（StackMust 自动采集）** |
| app.retryable / app.retry_count / app.upstream_service | 重试与上游上下文 |
| app.result | error |
