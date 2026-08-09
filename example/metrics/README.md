# example/metrics — 使用方自建指标

B5 定稿：库只给 `MeterProvider` + 配置模板；**指标语义归使用方**。

## 跑

```bash
# 可选：先起 LGTM
# docker compose -f observability/docker-compose.yml up -d

OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run .
# 或离线
OTEL_SDK_DISABLED=true go run .
```

## 代码在示范什么

| 步骤 | API | 说明 |
| --- | --- | --- |
| 1 | `telemetry.SetupFromEnvironment` | 装配含 Metric 的三信号 |
| 2 | `providers.Meter("example/shop")` | 使用方 meter 名 |
| 3 | `Float64Histogram("http.server.request.duration")` | RED 耗时（OTel 惯例名） |
| 4 | `Int64Counter("business.order.paid")` | 业务 counter（名称自定） |
| 5 | 低基数 attributes | 禁止 `user_id` / `order_id` 进标签 |

## Mimir / Grafana 查询（PromQL 示例）

OTel → Prometheus 名转换后大致为（以实际导出为准）：

```promql
# P99 延迟（按路由）
histogram_quantile(0.99,
  sum(rate(http_server_request_duration_seconds_bucket[5m])) by (le, http_route)
)

# 支付成功速率
sum(rate(business_order_paid_total[5m])) by (pay_channel)

# 错误率需使用方另埋 counter 或从 duration+status 推导
```

告警骨架见 `observability/templates/error-alerts.example.yaml`（与 log `error.type` 对齐，非强制 metric 名）。

面板联动：`observability/grafana/.../go-observability-overview.json` 中 Mimir 面板可把
`http.server.request.duration` 换成你的真实导出名后校准。
