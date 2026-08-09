# samber slog 互操作示例

此示例把项目的 `EventPayload.Attrs()` 送入 samber handler 链，演示 fan-out、格式化和采样。它不会替换仓库默认 Writer。

从仓库根目录运行：

```powershell
go run ./example/samber
Get-Content .\samber-out.jsonl
```

```bash
go run ./example/samber
tail -n 1 ./samber-out.jsonl
```

运行后 stdout 与仓库根目录的 `samber-out.jsonl` 都会收到记录。该文件是本地演示产物，不应提交。

注意 slog 自带的 level/timestamp 可能与事件属性重复；samber 的 PII formatter 和采样器也不能自动继承本项目的业务键清单与高价值结果规则。具体取舍见 [对比文档](../../docs/samber-comparison.md)。
