# 08 · 标准库 net/http

目标：不使用 Gin 时，用 `middleware/http` 装配完整链路，并用 10 行模板补 access 事件。

从仓库根目录运行：

```powershell
go run ./example/08_http
Invoke-WebRequest http://127.0.0.1:8081/healthz
Invoke-WebRequest -Method Post http://127.0.0.1:8081/api/v1/orders -Body '{}'
Get-Content .\logs\nethttp-events.jsonl
```

```bash
go run ./example/08_http
curl http://127.0.0.1:8081/healthz
curl -X POST http://127.0.0.1:8081/api/v1/orders -d '{}'
tail -n 1 ./logs/nethttp-events.jsonl
```

链路顺序：`Trace`（server span）→ `Recover`（panic 收口）→ access 模板 → `Metrics`（http.server.request.duration）→ `ErrorResponse`（显式错误收口）。

说明：库暂未提供 net/http access 中间件，本示例给出接入方包装模板；生产封装还应处理代理 IP 信任边界、请求 ID 传播和 Writer 错误监控。
