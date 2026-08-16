# 17 · 分层 span 调用链（handler→service→store→db）

演示 `middleware/otelutil` 的 `WithSpan` / `StartSpan` 如何把一次 HTTP 请求拆成
`server → handler → service → store → db` 的进程内父子 span 树（ADR-0023）。

## 运行

```powershell
go run ./example/17_layered_span
```

预期：标准输出以 pretty 格式打印 5 个 span，父子链
`server → handler → service → store → db`，共享同一 `trace_id`。

## 要点

- **server span** 由 `ginmw.Trace` 中间件创建（请求入口），span context 注入
  `c.Request.Context()`；
- **业务层**用 `otelutil.WithSpan(ctx, "service.order.Create", func(ctx context.Context) error { ... })`
  包一层函数：自动 Start/End，错误自动 `SetStatus(Error)+RecordError`，panic 标记后重抛；
- **ctx 逐层下传**是形成父子树的关键：每层必须使用 `WithSpan` 返回的新 ctx；
- `otelutil.WithStartOption(trace.WithAttributes(...))` 给 db 层 span 加
  `db.system=mysql`；
- 日志事件经 ctx 自动关联到当前层 span（`trace_id`/`span_id` 自动补全）。

## 验证

```powershell
go test ./example/17_layered_span -v
```

黑盒测试断言五层父子 span 树与 `db.system` 属性。