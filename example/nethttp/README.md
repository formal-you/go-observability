# 标准库 net/http 示例

演示不使用 Gin 时的完整链路：库中间件自动装配
`trace`（server span）/ `metrics`（http.server.request.duration）/ `nethttp`
（显式错误收口 `ErrorResponse` + panic 恢复 `Recover`）；access 事件由接入方
10 行包装（库暂无 net/http access 中间件，Gin 版见 `ginlog`；模板见 `accessLog`）。

从仓库根目录运行：

```powershell
go run ./example/nethttp
Invoke-WebRequest http://127.0.0.1:8081/healthz
Invoke-WebRequest -Method Post http://127.0.0.1:8081/api/v1/orders -Body '{}'
Get-Content .\logs
ethttp-events.jsonl
```

```bash
go run ./example/nethttp
curl http://127.0.0.1:8081/healthz
curl -X POST http://127.0.0.1:8081/api/v1/orders -d '{}'
tail -n 1 ./logs/nethttp-events.jsonl
```

成功路径写 `access.http.request`；错误路径额外写 `error.http.request`（业务拒绝 → WARN），
span/metrics 经全局 provider 输出（未配置 Collector 时 noop）。示例直接写 JSONL，不需要 Collector。
生产封装还应处理代理 IP 信任边界、请求 ID 传播和 Writer 错误监控。
