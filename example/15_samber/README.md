# 15 · samber slog 互操作

目标：把项目事件的 `EventPayload.Attrs()` 送入 samber handler 链，演示 fan-out、格式化与采样。

从仓库根目录运行：

```powershell
go run ./example/15_samber
Get-Content .\samber-out.jsonl
```

```bash
go run ./example/15_samber
tail -n 1 ./samber-out.jsonl
```

说明：samber 的 PII formatter 和采样器不会自动继承本项目的业务键清单与高价值结果规则。具体取舍见 [`docs/samber-comparison.md`](../../docs/samber-comparison.md)。
