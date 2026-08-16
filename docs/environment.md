# 环境变量速查

| 变量 | 读取位置 | 默认 | 行为 |
| --- | --- | --- | --- |
| `OTEL_SDK_DISABLED` | 应用映射到 `Config.Enabled` | 未设置，即启用 | 值为 `true` 时 `NewRuntime` 不创建 Provider；`Log.Output` 为空或 `otlp` 时回退 file Writer |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 应用映射到 `Config.Endpoint` | `127.0.0.1:4317` | 仅在 Trace/Metric/Log 中某个 `Output=otlp` 时解析；出口由各信号 Output 显式选择 |
| `GO_OBSERVABILITY_REGION` | 主示例 | 空 | 示例写入 `Resource.Region`；库不自动读取 |
| `GO_OBSERVABILITY_INSTANCE` | 主示例 | 空 | 示例映射到 `Resource.Instance`，写入 `service.instance.id`；库不自动读取 |

`OTEL_EXPORTER_OTLP_ENDPOINT` 接受 `host:port` 或完整 URL，内部会为无 scheme 的地址补 `http://`。TLS、证书、认证头等生产配置目前不由 `telemetry.Config` 暴露；需要这些能力时，应评估扩展 API 或在应用侧自建 Provider。

环境变量只在应用构造 Runtime 时读取；`Runtime.NewLogWriter` 使用已固化的 `Log.Output`，不会因运行中修改环境变量而切换目标。

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
