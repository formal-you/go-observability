# 06 · WithErrorHandler

演示 `log.WithErrorHandler`：观察 Writer 写入失败。

用一个「前 2 次成功、之后全部失败」的内存 `log.Writer`，展示：

- 写入失败时回调被**同步**触发，收到 `eventType` 与 `err`；
- 写失败**不会**作为业务返回值向上传播，也不中断后续事件写出；
- 实际写入成功后回调不会被调用。

从仓库根目录运行：

```powershell
go run ./example/06_errorhandler
```

预期输出：stdout 出现 `emitted event #1..#4`，stderr 出现两条 `error handler observed`，最后一行 `OK: ...`。
