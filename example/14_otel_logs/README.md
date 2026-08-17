# 14 · OTel Logs 双投影

目标：同一事件在 file 与 OTLP 两种出口下的投影差异，以及业务埋点为何不需要因出口变化而重写。

从仓库根目录运行本地 JSONL：

```powershell
go run ./example/14_otel_logs
Get-Content .\logs\otel-logs.jsonl
```

```bash
go run ./example/14_otel_logs
cat ./logs/otel-logs.jsonl
```

切到 OTLP（需先启动本地栈）：

```powershell
docker compose -f .\observability\docker-compose.yml up -d
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example/14_otel_logs
```

```bash
docker compose -f ./observability/docker-compose.yml up -d
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example/14_otel_logs
```

file/stdout 是扁平 JSONL；OTLP 把 timestamp、level、trace/span 映射到 LogRecord 顶层字段。完整字段映射见 [`docs/otel-logs-data-model.md`](../../docs/reference/otel-logs-data-model.md)。
