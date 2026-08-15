# 01 · 快速开始

目标：不依赖 Web 框架，用最小代码写出一条类型化业务事件。

从仓库根目录运行：

```powershell
go run ./example/01_quickstart
Get-Content .\logs\events.jsonl
```

```bash
go run ./example/01_quickstart
tail -n 1 ./logs/events.jsonl
```

预期输出：`logs/events.jsonl` 新增一条 `type=business` 的 JSONL，包含 `service.name`、`trace_id` / `span_id`、`event.name=order.payment.succeeded` 与 `app.result=success`。

关键 API：

- `telemetry.NewFileRuntime`：小单体 file-only 预设；
- `runtime.NewWriter`：创建统一日志 Writer；
- `log.NewLogger` + `WithTraceExtractor` + `WithErrorHandler`：装配 Logger；
- `Logger.Emit`：写出类型化事件。

这是教学最小骨架。生产代码请把文件路径、服务身份和错误处理策略放入配置。
