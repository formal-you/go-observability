# 待办事项

本文件只记录跨工具的流程待办，不复制 GitHub Issue 的技术验收清单。技术需求与进度仍以 GitHub Issues 为准。

## 飞书项目管理迁移准备

- [ ] 选择飞书 Tasks（轻量任务）还是 Meegle（项目工作项、迭代和排期）作为主模型。
- [ ] 使用官方 `lark-cli` 完成应用配置与 OAuth 登录；不要把 App ID、App Secret 或 token 写入仓库。
- [ ] 设计 GitHub Issue ↔ 飞书工作项的字段映射：标题、状态、负责人、优先级、验收链接、PR 链接。
- [ ] 先以 GitHub Issues 作为唯一技术真源，完成一轮只读同步验证，再决定是否允许双向更新。
- [ ] 为同步失败、重复创建、权限不足和敏感字段泄露定义回滚与审计规则。

当前已安装官方 `lark-cli` `1.0.85`（Windows 包 `ByteDance.LarkCLI`）。首次使用需人工执行：

```powershell
lark-cli config init --new
lark-cli auth login --recommend
lark-cli auth status
```

认证需要浏览器授权，未授权前 CLI 不能访问飞书数据；本仓库不保存任何认证状态。
