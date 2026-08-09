# Examples

| 目录 | 说明 |
| --- | --- |
| [config/](config/) | **配置样例（字段注释）**：`.env` / app YAML / collector / compose |
| [.](.) `main.go` | Gin + telemetry + ginlog 端到端 |
| [mall/](mall/) | 接入方 business 事件注册表（C2） |
| [metrics/](metrics/) | 使用方自建 Meter（B5） |
| [nethttp/](nethttp/) | 无 Gin 的 access 埋点（C3） |
| [samber/](samber/) | 与 samber 生态对照 spike |

## 推荐本地路径

```bash
# 1)（可选）起 LGTM
docker compose -f observability/docker-compose.yml up -d

# 2) 复制并编辑环境变量（字段说明见文件内注释）
cp example/config/.env.example example/config/.env
# 编辑 example/config/.env 后：
#   export $(grep -v '^#' example/config/.env | xargs)   # bash
#   或在 PowerShell 中逐条 $env:NAME="value"

# 3) 跑演示
go run ./example
# JSONL 默认：example/logs/events.jsonl（未设 OTEL_EXPORTER_OTLP_ENDPOINT 时）
```

配置语义总览：[docs/configuration.md](../docs/configuration.md)。
