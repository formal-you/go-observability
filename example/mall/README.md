# mall · 领域事件注册表 + 端到端参考服务

本目录分两层：

- `events.go`：接入方自建的 Event Registry（事件名 + `app.*` 键 + `ExtraAttrs` 构造器）；
- `cmd/`：端到端小商城服务，把注册表、错误体系、治理与 Gin 全链路中间件组合起来。

## 验证注册表

从仓库根目录运行：

```bash
go test ./example/mall
```

## 运行端到端服务

```powershell
go run ./example/mall/cmd
Invoke-WebRequest http://127.0.0.1:8083/api/v1/products/42
Invoke-WebRequest -Method Post http://127.0.0.1:8083/api/v1/orders -Headers @{ 'X-Fail' = 'stock' }
Get-Content .\logs\mall.jsonl
```

```bash
go run ./example/mall/cmd
curl http://127.0.0.1:8083/api/v1/products/42
curl -X POST -H 'X-Fail: stock' http://127.0.0.1:8083/api/v1/orders
tail -n 1 ./logs/mall.jsonl
```

服务展示：`mall.EventProductViewed` / `EventOrderCreated`、业务拒绝错误、`SecurityLog` / `AuditLog`、Sampler / Masker 治理与三信号装配。
