# Metric 命名惯例模板（使用者自有，非库契约）

> go-observability **不** 把下列名称写进核心 API。接入方按 OTel semantic conventions
> 与自身领域命名；直方图桶按 SLA 校准。

## 1. RED（HTTP / RPC）

| 意图 | 建议名（OTel 惯例） | 类型 | 常用维度（低基数） |
| --- | --- | --- | --- |
| 请求耗时 | `http.server.request.duration` | Histogram (s) | `http.request.method`, `http.route`, `http.response.status_code` |
| 主动出站 | `http.client.request.duration` | Histogram (s) | `server.address`, `http.response.status_code` |
| 错误计数 | 由 duration + status 推导，或 `http.server.request.errors` Counter | Counter | 同上 + `error.type`（若有） |

直方图桶示例（秒，覆盖 p50/p90/p99，按 SLA 改）：

```text
0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
```

## 2. USE（进程 / 运行时）

优先用 OTel runtime / host 自动 instrumentation，不必手埋：

- `process.runtime.go.*`（GC、goroutine）
- `system.cpu.*` / `system.memory.*`（若开 host metrics）

## 3. 依赖

| 意图 | 建议名 | 类型 |
| --- | --- | --- |
| DB | `db.client.operation.duration` | Histogram |
| 缓存 | `db.client.operation.duration` + `db.system=redis` | Histogram |
| 消息 | `messaging.client.operation.duration` | Histogram |

## 4. 业务指标（示例，非强制）

业务 counter/histogram **只在应用服务内创建**，命名建议：

```text
{domain}.{entity}.{action}  # 如 order.paid / payment.refund.success
```

单位：金额用最小货币单位整数；比率用 0–1 或单独 success/total counter。

与日志事件对齐时：可用同一 `event.name` / `error.type` 作标签，但 **禁止** 高基数
（user_id、order_id 不得进 metric 标签）。

## 5. Exemplar

若后端支持（Mimir + Grafana），在 histogram 上开 exemplar，实现 metric → trace 跳转
（设计 A4）。桶与导出间隔由接入方 `PeriodicReader` / Collector batch 决定。
