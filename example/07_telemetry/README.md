# 07 · Telemetry 预设

目标：理解四种 `Runtime` 预设分别装配哪些信号，以及何时应改用 `NewRuntime`。

从仓库根目录运行：

```powershell
go run ./example/07_telemetry -mode=file
go run ./example/07_telemetry -mode=log
go run ./example/07_telemetry -mode=all-file
go run ./example/07_telemetry -mode=otlp -endpoint=127.0.0.1:4317
```

| 模式 | 预设 | 输出 |
| --- | --- | --- |
| `file` | `NewFileRuntime` | `logs/telemetry-events.jsonl` |
| `log` | `NewLogRuntime` | `logs/telemetry-log.jsonl` |
| `all-file` | `NewAllFileRuntime` | `logs/telemetry-all/{events,trace,metric}.jsonl` |
| `otlp` | `NewOTLPRuntime` | Collector（需先启动本地栈） |

需要自定义采样比例、批量导出间隔、队列或轮转时，改用 `telemetry.NewRuntime` 与分信号 `Config`。
