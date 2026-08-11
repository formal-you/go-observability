# 环境变量速查

| 变量 | 读取位置 | 默认 | 行为 |
| --- | --- | --- | --- |
| `OTEL_SDK_DISABLED` | 应用映射到 `Config.Enabled` | 未设置，即启用 | 值为 `true` 时返回不含 Provider 的 Runtime；兼容 `SetupFromEnvironment` 仍回退 JSONL |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 应用映射到 `Config.Endpoint` | `127.0.0.1:4317` | `LogOutputOTLP` 使用该 OTLP gRPC 地址；出口由 `Config.LogOutput` 显式选择 |
| `GO_OBSERVABILITY_REGION` | 主示例 | 空 | 示例写入 resource `region`；库不自动读取 |
| `GO_OBSERVABILITY_INSTANCE` | 主示例 | 空 | 示例写入 resource `instance`；库不自动读取 |

`OTEL_EXPORTER_OTLP_ENDPOINT` 接受 `host:port` 或完整 URL，内部会为无 scheme 的地址补 `http://`。TLS、证书、认证头等生产配置目前不由 `telemetry.Config` 暴露；需要这些能力时，应评估扩展 API 或在应用侧自建 Provider。

环境变量只在应用构造 Runtime 时读取；`Runtime.NewWriter` 使用已固化的出口决策，不会因运行中修改环境变量而切换目标。`Setup*` 与 `NewLogWriter` 仅是 Deprecated 兼容入口。

PowerShell：

```powershell
$env:OTEL_SDK_DISABLED = "true"
go run ./example
Remove-Item Env:OTEL_SDK_DISABLED
```

bash：

```bash
OTEL_SDK_DISABLED=true go run ./example
```

标准 OTel 环境变量并非都会被本库读取。以 `telemetry.Config`、本表和源码为准，不要假设 SDK 支持的任意变量都已接入。
