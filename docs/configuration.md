# 配置指南

本文说明 **应用如何配置 go-observability**，以及 **部署侧配置落在哪里**。

## 1. 配置分层

| 层 | 谁拥有 | 载体 |
| --- | --- | --- |
| 应用代码 | 接入方 | `telemetry.Config`、`log.NewLogger` 选项、事件埋点 |
| 进程环境 | 接入方 / 平台 | `OTEL_*`、`GO_OBSERVABILITY_*`（见 [example/config/.env.example](../example/config/.env.example)） |
| Collector / 存储 | 运维 | [example/config/collector.example.yaml](../example/config/collector.example.yaml)、[observability/](../observability/) |
| 告警 / 看板 | 运维 | [observability/templates/](../observability/templates/)、Grafana provisioning |

库 **不** 内置配置文件解析器；YAML 样例仅作字段文档。

## 2. 环境变量（库实际读取）

| 变量 | 读取点 | 行为 |
| --- | --- | --- |
| `OTEL_SDK_DISABLED=true` | `EnabledFromEnvironment` | 空 Providers，三信号 noop |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `EndpointFromEnvironment` / `NewLogWriter` | Setup 默认 endpoint；**非空** 且已启用 → 日志走 OTLP，否则 JSONL |
| `GO_OBSERVABILITY_REGION` | example（可选） | 写入 resource `region` |
| `GO_OBSERVABILITY_INSTANCE` | example（可选） | 写入 resource `instance` |

完整注释清单：[example/config/.env.example](../example/config/.env.example)。

## 3. telemetry.Config 字段

| 字段 | 必填 | 默认 | 含义 |
| --- | --- | --- | --- |
| `ServiceName` | 是 | — | `service.name` |
| `ServiceVersion` | 否 | `""` | `service.version` |
| `Environment` | 否 | `dev` | `deployment.environment` |
| `Region` / `Instance` | 否 | 省略 | 低基数标签 |
| `Endpoint` | 否 | env 或 `127.0.0.1:4317` | OTLP gRPC |
| `Enabled` | 否 | env | 是否装配 SDK |
| `TraceSampleRatio` | 否 | `0.1` | 头部采样 **(0,1]** |
| `TraceBatchTimeout` | 否 | `5s` | span 批导出 |
| `MetricExportInterval` | 否 | `15s` | metric 周期 |
| `LogBatchTimeout` | 否 | `1s` | log 批导出 |
| `Resource` | 否 | 由上列构建 | 自定义 Resource |

结构示意 YAML：[example/config/app.example.yaml](../example/config/app.example.yaml)。

## 4. Logger 选项

```go
log.NewLogger(w,
    log.WithSampler(log.ResultKeepSampler{Ratio: 1}), // 高价值 result 必留
    log.WithMasker(log.FieldMasker{Keys: []string{"app.phone"}}),
    log.WithBaseMetadata(log.EventMetadata{/* ... */}),
    log.WithErrorHandler(func(ctx context.Context, msg string, attrs []slog.Attr, err error) { /* ... */ }),
)
```

| 选项 | 默认 | 说明 |
| --- | --- | --- |
| Sampler | 无（全量写） | `ResultKeepSampler`：failed/error/blocked/denied 必留 |
| Masker | 无 | `FieldMasker`：默认密钥类键 → `***` |
| BaseMetadata | 空 | 补全缺失的 level/trace/span/request/latency |
| ErrorHandler | 无 | Writer 失败观察，不返回业务 |

## 5. B9 出口决策矩阵

| `OTEL_SDK_DISABLED` | `OTEL_EXPORTER_OTLP_ENDPOINT` | Trace/Metric | Log Writer |
| --- | --- | --- | --- |
| `true` | * | noop | 建议 file/stdout |
| 非 true | 空 | 若 Enabled 仍可能连默认 endpoint* | **JSONL**（`NewLogWriter`） |
| 非 true | 非空 | OTLP | **OTLP** |

\*本地推荐：未就绪时 `OTEL_SDK_DISABLED=true`，或只跑 JSONL 且不调用需要真实 exporter 的路径。

## 6. 部署侧

1. 复制 [example/config/collector.example.yaml](../example/config/collector.example.yaml) 或使用 [observability/otel-collector-config.yaml](../observability/otel-collector-config.yaml)。
2. `docker compose -f observability/docker-compose.yml up -d`（需 Docker）。
3. 告警骨架：`observability/templates/error-alerts.example.yaml`（阈值自定）。
4. Metric 名/桶：**接入方自定**（B5），见 `templates/metric-names.example.md`。

## 7. 相关文档

- [environment.md](environment.md) — 环境变量速查
- [security.md](security.md) — 脱敏与密钥
- [onboarding.md](onboarding.md) — 上手
- [architecture.md](architecture.md) — 架构
