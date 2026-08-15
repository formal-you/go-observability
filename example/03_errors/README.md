# 03 · 错误建模与投影

目标：理解 `errs` 的 `ErrorKind` / `ErrorType` / `ErrorCode` 边界，以及 `log.EventFromError` 如何把 error 投影成事件。

从仓库根目录运行：

```powershell
go run ./example/03_errors
Get-Content .\logs\errors.jsonl
```

```bash
go run ./example/03_errors
cat ./logs/errors.jsonl
```

预期输出：

- stdout 打印四条错误的 `LevelOf` / `Kind` / `Code` / `Type` / `IsRetryExhausted`；
- `logs/errors.jsonl` 出现 2 条 `type=business`（validation / business）和 2 条 `type=error`（system）。

关键 API：

- `errs.NewBusinessError` / `NewValidationError` / `NewSystemError`：严格构造；
- `errs.MustRegisterErrorCode` / `MustRegisterErrorContract`：启动期固定 code → type 映射；
- `log.EventFromError`：按 `ErrorKind` 分派到 BusinessEvent 或 ErrorEvent；
- `log.LevelOf`：错误缺省级别推导。
