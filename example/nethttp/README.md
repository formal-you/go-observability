# 标准库 net/http 示例

核心仓库只提供 Gin 中间件。本示例演示不引入 Gin 时，如何在 `net/http` handler 中构造 `AccessEvent`。

从仓库根目录运行：

```powershell
go run ./example/nethttp
Invoke-WebRequest http://127.0.0.1:8081/healthz
Get-Content .\logs\nethttp-events.jsonl
```

```bash
go run ./example/nethttp
curl http://127.0.0.1:8081/healthz
tail -n 1 ./logs/nethttp-events.jsonl
```

示例直接写 JSONL，不需要 Collector。生产封装还应处理真实状态码记录、代理 IP 信任边界、请求 ID 传播和 Writer 错误监控。
