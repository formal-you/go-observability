# 05 · 框架中间件接入：Gin、net/http、gRPC、kratos

## 一、痛点

中间件是所有 Go 服务接入可观测性的第一站，但很多团队：

- 每换一个框架就重写一次 trace / metrics / error 收口；
- access 日志和 panic 恢复的顺序容易写错；
- 错误响应体不统一，内部 message 还可能泄露。

`go-observability` 把错误契约核心抽到 `middleware/httperr`，再为各框架做薄适配。

## 二、Gin 全链路

从仓库根目录运行：

```powershell
go run ./example/09_gin
Invoke-WebRequest http://127.0.0.1:8080/api/v1/products/42
Get-Content .\logs\events.jsonl
```

```bash
go run ./example/09_gin
curl http://127.0.0.1:8080/api/v1/products/42
tail -n 1 ./logs/events.jsonl
```

中间件顺序即执行顺序：

```text
Trace -> AccessLog -> Recover -> Metrics -> ErrorResponse
```

`AccessLog` 必须包在 `Recover` 外层，才能在 panic 被收口后记录最终 500 响应。

## 三、net/http

从仓库根目录运行：

```powershell
go run ./example/08_http
Invoke-WebRequest http://127.0.0.1:8081/healthz
```

`middleware/http` 提供 `Trace` / `Recover` / `Metrics` / `ErrorResponse`；access 事件由接入方用一个 10 行模板包装。

## 四、gRPC

从仓库根目录运行：

```powershell
go run ./example/10_grpc
```

`grpcmw.Trace` 与 `grpcmw.Metrics` 是 unary 拦截器：

```go
grpc.NewServer(grpc.ChainUnaryInterceptor(
    grpcmw.Trace(grpcmw.TraceConfig{}),
    grpcmw.Metrics(grpcmw.MetricsConfig{}),
))
```

## 五、kratos

从仓库根目录运行：

```powershell
go run ./example/11_kratos
Get-Content .\logs\kratos.jsonl
```

kratos 适配提供：

- `ErrorEncoder`：HTTP 错误编码；
- `ErrorLog`：错误事件日志；
- `GRPCErrorMapper`：gRPC status 映射。


## 代码对比：Gin 错误收口

传统做法通常每个 handler 自己决定状态码和响应体：

```go
if bizErr {
    c.JSON(409, gin.H{"code": "STOCK_INSUFFICIENT", "message": "库存不足"})
    return
}
```

`go-observability`：

```go
router.Use(ginmw.ErrorResponse(ginmw.ErrorConfig{
    Logger: logger,
    EventNameResolver: func(err error) log.EventName {
        return log.NewEventName("order", "create", "stock_insufficient")
    },
}))

// handler 只挂载错误：
ginmw.Abort(c, bizErr)
```

状态码、响应体和错误事件统一在收口中间件处理，handler 不再各写一套。

## 六、最佳实践

1. 错误收口只做“判定 + 渲染”，具体事件名由接入方 resolver 提供；
2. 系统错误不透传内部 message，只给固定文案；
3. 各框架共享 `httperr` 契约，避免同一错误在不同框架下行为不一致；
4. access 日志把健康检查 `SkipPaths` 排除。

下一篇：[06 · Security/Audit 审计留痕](06-security-audit.md)。
