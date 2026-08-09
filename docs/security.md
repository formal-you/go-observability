# 安全与脱敏

## 责任边界

| 层 | 职责 |
| --- | --- |
| 应用 | 入口脱敏、`FieldMasker` / 自定义 `Masker`、不把密钥打进日志 |
| Collector | 二次兜底 transform（可选，见 collector 样例注释） |
| 存储 / 访问控制 | 审计独立、权限、保留期（接入方） |

本库 **不是** 合规产品；默认 Masker 仅覆盖常见密钥键名。

## FieldMasker

```go
log.NewLogger(w, log.WithMasker(log.FieldMasker{
    Keys:   []string{"app.phone", "app.id_card"}, // 追加 PII
    Redact: "***",
}))
```

- 默认键：`DefaultSensitiveKeys`（password、token、authorization、api_key…）
- 匹配：精确键（大小写不敏感）或后缀 `*.token` / `*_password`
- Group：递归
- **默认不含** `app.user_id`（业务标识）；需要时自行加入 `Keys`

## Sampler 与噪音

- `ResultKeepSampler`：高价值 `app.result` 必留，避免采样丢掉故障面
- 生产 debug 级流量请用级别 + ratio 双控

## 密钥与仓库

- 禁止把真实 token、云密钥提交进 git
- CI 跑 `govulncheck`（见 `.github/workflows/ci.yml`）
- 漏洞报告见 [SECURITY.md](../SECURITY.md)

## Trace 采样

默认 `TraceSampleRatio=0.1` 为**演示值**。生产按流量与合规（是否允许丢 trace）调整；严格错误必采可 ratio=1 或 Collector `tail_sampling`。
