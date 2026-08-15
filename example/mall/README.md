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
Invoke-WebRequest -Method Post 'http://127.0.0.1:8083/api/v1/orders?product_id=42&quantity=999' -Headers @{ 'X-Fail' = 'stock' }
Get-Content .\logs\mall.jsonl
```

```bash
go run ./example/mall/cmd
curl http://127.0.0.1:8083/api/v1/products/42
curl -X POST -H 'X-Fail: stock' 'http://127.0.0.1:8083/api/v1/orders?product_id=42&quantity=999'
tail -n 1 ./logs/mall.jsonl
```

## 触发各类事件

```powershell
# 安全事件：任意路径带 X-Risk: high 时 SecurityLog 写 auth.login.denied
Invoke-WebRequest http://127.0.0.1:8083/healthz -Headers @{ 'X-Risk' = 'high' }

# 审计事件：actor 来自认证上下文（X-User-ID / X-User-Role），记录 before/after 与来源
Invoke-WebRequest -Method Post http://127.0.0.1:8083/api/v1/admin/users/42/role -Headers @{
    'X-User-ID'   = 'u_admin'
    'X-User-Role' = 'admin'
    'X-Request-ID'= 'req-audit-1'
}

# 业务拒绝：ErrorResponse 写 business 错误事件，mallInputGuard 再补发 audit 事件；
# audit 会携带失败输入摘要（app.input_field 等）与低敏参数值（app.product_id / app.quantity）
Invoke-WebRequest -Method Post 'http://127.0.0.1:8083/api/v1/orders?product_id=42&quantity=999' -Headers @{ 'X-Fail' = 'stock' }
```

审计事件现在包含 `app.before` / `app.after`、`client.address`、`user_agent.original`，
且 `app.actor_user_id` / `app.actor_role` 由 `fakeIdentity` 注入可信上下文生成。
库存不足失败路径会同时产生 `order.create.stock_insufficient`（business）与
`input.anomaly.recorded`（audit），后者携带 `app.input_field` / `app.input_hash` /
`app.input_truncated` 以及低敏参数值 `app.product_id` / `app.quantity`。

服务展示：`mall.EventProductViewed` / `EventOrderCreated`、业务拒绝错误、`SecurityLog` / `AuditLog`、Sampler / Masker 治理与三信号装配。
