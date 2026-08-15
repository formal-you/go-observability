# 12 · Security 与 Audit 中间件

目标：演示 Gin 的 `SecurityLog` / `AuditLog`，以及回调式 `Decide` / `Describe` 契约。

从仓库根目录运行：

```powershell
go run ./example/12_security_audit
Get-Content .\logs\security-audit.jsonl
```

```bash
go run ./example/12_security_audit
cat ./logs/security-audit.jsonl
```

预期输出：两个 HTTP 请求各返回 200；`logs/security-audit.jsonl` 新增一条 `type=security` 与一条 `type=audit`。

推荐模式：认证/授权中间件只通过 `Decide` / `Describe` 给出判定，库只负责写成事件。深层代码需要挂载载荷时使用 `ginmw.SetSecurity` / `ginmw.SetAudit`。
