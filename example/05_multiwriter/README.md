# 05 · MultiWriter 多出口

目标：用 `log.NewMultiWriter` 把同一个事件 fan-out 到 file 与 stdout，对比两种投影。

从仓库根目录运行：

```powershell
go run ./example/05_multiwriter
Get-Content .\logs\multiwriter.jsonl
```

```bash
go run ./example/05_multiwriter
cat ./logs/multiwriter.jsonl
```

预期输出：

- stdout 收到 OTel LogRecord 投影；
- `logs/multiwriter.jsonl` 收到扁平 JSONL。

任一 Writer 失败不会阻断其余 Writer；关闭时 `ManagedWriter.Close` 会尝试关闭全部可关闭子 Writer。
