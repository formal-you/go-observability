# 04 · Sampler 与 Masker 治理

目标：演示 `Logger` 管线的两个显式注入点——`Masker`（脱敏）与 `Sampler`（采样）。

从仓库根目录运行：

```powershell
go run ./example/04_sampler_masker
Get-Content .\logs\governance.jsonl
```

```bash
go run ./example/04_sampler_masker
cat ./logs/governance.jsonl
```

预期输出：`logs/governance.jsonl` 只有 3 条事件：

- `type=error`
- `type=security`
- `type=audit`

被 `Fallback` 丢弃的是 `access` 与 `business`。`type=error` 中 `app.secret` 已被替换为 `***`。

治理顺序固定为：metadata 补全 → 最低级别过滤 → Masker → Sampler → Write。生产环境请把高价值失败/安全/审计强制保留，成功访问噪音显式采样。
