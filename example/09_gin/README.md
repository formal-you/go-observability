# 09 · Gin 全链路

目标：在 Gin 服务里一次装配 Trace、Access、Recover、Metrics、ErrorResponse 全链路中间件，并演示按环境切换信号出口。

从仓库根目录运行：

```powershell
go run ./example/09_gin
Invoke-WebRequest http://127.0.0.1:8080/api/v1/products/42
Invoke-WebRequest -Method Post http://127.0.0.1:8080/api/v1/orders -Body '{}'
Get-Content .\logs\events.jsonl
```

```bash
go run ./example/09_gin
curl http://127.0.0.1:8080/api/v1/products/42
curl -X POST http://127.0.0.1:8080/api/v1/orders -d '{}'
tail -n 1 ./logs/events.jsonl
```

设置 `OTEL_EXPORTER_OTLP_ENDPOINT` 后，三信号统一改走 OTLP。关键 API：`ginmw.Trace` / `AccessLog` / `Recover` / `Metrics` / `ErrorResponse` / `Abort`。
