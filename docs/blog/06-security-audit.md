# 06 · Security/Audit 审计留痕：让安全判定自动变成事件

## 一、痛点

安全与审计日志往往被做成“事后补记”：

- 谁在什么时间、因为什么被拦截，散落在业务代码里；
- 高权限操作只记了“成功”，没记 before/after 和审批信息；
- 安全事件与请求 trace 关联不上，事后复盘困难。

`go-observability` 的 `SecurityLog` / `AuditLog` 让认证/授权边界只负责判定，库只负责写成事件。

## 二、回调式契约

推荐使用回调式 `Decide` / `Describe`：

- `SecurityConfig.Decide`：认证/授权中间件返回 `*SecurityPayload`，nil 表示不记录；
- `AuditConfig.Describe`：在链尾读取最终状态，返回 `*AuditPayload`。

这样安全判定逻辑留在接入方，事件结构由库统一。

## 三、动手

从仓库根目录运行：

```powershell
go run ./example/12_security_audit
Get-Content .\logs\security-audit.jsonl
```

```bash
go run ./example/12_security_audit
cat ./logs/security-audit.jsonl
```

示例会触发两个请求：

- 登录请求带 `X-Risk: high`，产出 `type=security`；
- 管理员改角色请求，产出 `type=audit`。

## 四、输出解读

`security` 事件默认 WARN，`audit` 事件默认 INFO。审计事件包含：

```text
app.action        = admin.role_update
app.actor_user_id = u_admin
app.actor_role    = admin
app.resource_type = user
app.resource_id   = u-2002
app.target_user_id = u-2002
app.changed_fields = ["role"]
app.before / app.after
app.reason
```

这些字段用于追溯“谁在何时对什么资源做了什么”，不是普通访问日志。

## 五、最佳实践

1. 安全判定放在认证/授权中间件，不散落在 handler；
2. 审计只针对高权限操作与敏感资源变更，避免刷量；
3. `before/after` 记录字段级变化，`reason` 记录业务原因；
4. 安全事件可直连 SIEM，审计事件进入防篡改存储。

下一篇：[07 · JSONL 与 OTLP 双投影](07-jsonl-vs-otlp.md)。
