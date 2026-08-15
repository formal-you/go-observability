# 07 · JSONL 与 OTLP 双投影：同一事件，两种形态

## 一、痛点

本地排障喜欢 JSONL，可观测平台喜欢 OTLP：

- JSONL 方便 `grep` / `tail` / 回放；
- OTLP 有标准的 LogRecord 顶层字段，方便 Collector/Loki/Tempo 处理；
- 如果两套逻辑分开维护，字段迟早漂移。

`go-observability` 让业务代码只写一次事件，由 Writer 决定投影形态。

## 二、动手

从仓库根目录运行本地 JSONL：

```powershell
go run ./example/14_otel_logs
Get-Content .\logs\otel-logs.jsonl
```

```bash
go run ./example/14_otel_logs
cat ./logs/otel-logs.jsonl
```

切到 OTLP 需要 Collector：

```powershell
docker compose -f .\observability\docker-compose.yml up -d
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example/14_otel_logs
```

## 三、映射关系

| 事件字段 | file/stdout JSONL | OTLP LogRecord |
| --- | --- | --- |
| 时间 | `timestamp` 扁平列 | `Timestamp` 顶层字段 |
| 级别 | `level` 扁平列 | `SeverityNumber` / `SeverityText` |
| 事件名 | `event.name` 扁平列 | `EventName` 顶层字段 |
| trace/span | `trace_id` / `span_id` | `TraceId` / `SpanId` |
| 业务属性 | `app.*` 扁平列 | `Attributes` |

## 四、为什么业务代码不用改

`Runtime.NewWriter(ctx)` 会根据 `LogConfig.Output` 返回对应 Writer：

```go
Log: telemetry.LogConfig{
    Output:   telemetry.SignalOutputFile, // 或 SignalOutputOTLP
    FilePath: "logs/otel-logs.jsonl",      // 仅 file 模式需要
},
```

业务侧始终是同一个 `Logger.Emit`，出口切换只发生在装配层。

## 五、最佳实践

1. 不要把出口选择写进业务代码；
2. 本地开发用 file/stdout，生产用 OTLP；
3. 用同一份事件模型保证两边的字段一致；
4. 查看完整字段映射文档 `docs/otel-logs-data-model.md`。

下一篇：[08 · mall 端到端诊断故事](08-mall-end-to-end.md)。
