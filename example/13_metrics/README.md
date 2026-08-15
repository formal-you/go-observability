# 13 · 使用方自定义指标

目标：演示使用方如何用 `Runtime.Meter` 自建业务指标，并把 `Trace` / `Metric` / `Log` 分开装配。

离线验证程序可启动：

```powershell
$env:OTEL_SDK_DISABLED = "true"
go run ./example/13_metrics
```

```bash
OTEL_SDK_DISABLED=true go run ./example/13_metrics
```

向本地 Collector 导出：

```powershell
docker compose -f .\observability\docker-compose.yml up -d
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example/13_metrics
```

```bash
docker compose -f ./observability/docker-compose.yml up -d
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example/13_metrics
```

不要把 `user_id`、`order_id` 等高基数字段放入 Metric attributes；这会显著增加时序数量和成本。服务端 RED 指标由 `middleware/gin` / `middleware/http` 自动记录，本示例只演示使用方自定义指标。
