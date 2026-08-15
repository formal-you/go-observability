# 02 · 六类类型化事件

目标：一次看懂 `log` 包的六类事件，以及 `app.*` 扩展字段如何扁平进入 JSONL。

从仓库根目录运行：

```powershell
go run ./example/02_events
Get-Content .\logs\events-six.jsonl
```

```bash
go run ./example/02_events
cat ./logs/events-six.jsonl
```

| `type` | 载荷 | 典型场景 |
| --- | --- | --- |
| `access` | `AccessPayload` | 每请求一条，HTTP/RPC 访问与延迟 |
| `business` | `BusinessPayload` | 业务动作、业务拒绝 |
| `error` | `ErrorPayload` | 系统错误、panic、依赖失败 |
| `security` | `SecurityPayload` | 认证、鉴权、风控拦截 |
| `audit` | `AuditPayload` | 高权限操作与敏感资源变更 |
| `probe` | `ProbePayload` | 健康、就绪、存活探测 |

关键 API：`log.AccessEvent` / `BusinessEvent` / `ErrorEvent` / `SecurityEvent` / `AuditEvent` / `ProbeEvent`，以及 `BusinessPayload.ExtraAttrs` 注入 `app.*` 键。
