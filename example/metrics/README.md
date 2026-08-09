# 使用方指标示例

本库提供 `telemetry.Providers.Meter`，但不替使用方定义业务指标。本示例创建 HTTP 耗时直方图和支付计数器，并展示低基数属性。

离线验证程序可启动：

```powershell
$env:OTEL_SDK_DISABLED = "true"
go run ./example/metrics
```

```bash
OTEL_SDK_DISABLED=true go run ./example/metrics
```

向本地 Collector 导出：

```powershell
docker compose -f .\observability\docker-compose.yml up -d
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example/metrics
```

```bash
docker compose -f ./observability/docker-compose.yml up -d
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example/metrics
```

指标名称和 Prometheus 转换结果以实际 Collector/Exporter 版本为准。不要把 `user_id`、`order_id` 等高基数字段放入 Metric attributes；这会显著增加时序数量和成本。
