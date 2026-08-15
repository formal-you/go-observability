# 02 · 错误建模与投影：error 不要被日志层二次丢失

## 一、痛点

Go 的 `error` 常被 `%w` 包装，到日志层时你已经不知道它原本是什么：

- 是参数错误还是系统故障？
- 该返回 400 还是 500？
- 该不该告警、要不要重试？
- 重试了几次、上游是谁？

`go-observability` 把错误拆成三层，并在日志投影时保留这些信息。

## 二、三层模型

| 层 | 作用 | 示例 |
| --- | --- | --- |
| `ErrorKind` | 预期性分类 | `validation` / `business` / `system` |
| `ErrorType` | OTel/gRPC 标准枚举 | `FAILED_PRECONDITION` / `DEADLINE_EXCEEDED` |
| `ErrorCode` | 具体业务码 | `ORDER.CREATE.STOCK_INSUFFICIENT` |

- `ErrorKind` 决定收口层状态码与是否告警；
- `ErrorType` 低基数、闭合枚举，用于聚合和路由；
- `ErrorCode` 稳定具体错误码，用于检索和契约测试。

## 三、动手

从仓库根目录运行：

```powershell
go run ./example/03_errors
Get-Content .\logs\errors.jsonl
```

```bash
go run ./example/03_errors
cat ./logs/errors.jsonl
```

stdout 会先打印分类结果：

```text
biz      level=WARN kind=business code=ORDER.CREATE.STOCK_INSUFFICIENT type=FAILED_PRECONDITION
valid    level=WARN kind=validation type=INVALID_ARGUMENT
system   level=WARN kind=system code=INFRA.MYSQL.QUERY_TIMEOUT type=DEADLINE_EXCEEDED retry_exhausted=false
exhaust  level=ERROR retry_exhausted=true
```

## 四、投影规则

`log.EventFromError` 沿错误链提取 `errs.AppError`，按 `Kind` 分派：

- `validation` / `business` → `BusinessEvent`；
- `system` / 普通 error → `ErrorEvent`。

错误事件名必须由接入方从注册表传入，框架不自动派生泛化错误事件名，避免“万能错误”让告警失效。


## 代码对比：记录一个业务错误

传统写法容易丢失错误分类：

```go
if err != nil {
    slog.Error("create order failed", "error", err.Error())
}
```

`go-observability`：

```go
biz, buildErr := errs.NewBusinessError(errs.BusinessErrorConfig{
    Code:    "ORDER.CREATE.STOCK_INSUFFICIENT",
    Type:    errs.TypeFailedPrecondition,
    Message: "商品库存不足",
})
if buildErr != nil {
    return buildErr
}
logger.Emit(ctx, log.EventFromError(
    log.NewEventName("order", "create", "stock_insufficient"),
    biz,
    log.EventMetadata{},
))
```

后者保留 `ErrorKind` / `ErrorType` / `ErrorCode`，收口层能自动映射状态码和告警级别。

## 五、最佳实践

1. 进程启动期用 `MustRegisterErrorCode` 固定 `ErrorCode -> ErrorType` 映射；
2. 业务拒绝用 `NewBusinessError`，参数校验用 `NewValidationError`，非预期故障用 `NewSystemError`；
3. 重试耗尽、不可重试的系统错误按 ERROR 处理，重试中按 WARN 处理；
4. 系统错误不透传内部 message，细节只进日志事件，不返回客户端。

下一篇：[03 · 日志治理：采样、脱敏、多出口](03-log-governance.md)。
