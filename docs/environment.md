# 环境变量速查

| 变量 | 组件 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| `OTEL_SDK_DISABLED` | telemetry | 否 | 未设置=启用 | `true` → 空 Providers |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | telemetry / NewLogWriter | 否 | Setup 默认 `127.0.0.1:4317`；Writer 以「是否设置」分支 | host:port 或 URL |
| `GO_OBSERVABILITY_REGION` | example | 否 | 空 | resource `region` |
| `GO_OBSERVABILITY_INSTANCE` | example | 否 | 空 | resource `instance` |

带逐字段注释的可复制文件：[example/config/.env.example](../example/config/.env.example)。

标准 OTel 变量（如 `OTEL_SERVICE_NAME`）本库 **优先** 使用 `telemetry.Config` 显式字段；未接入的变量行为以 OTel Go SDK 为准。
