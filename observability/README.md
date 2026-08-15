# 本地 LGTM 参考栈

本目录提供开发和演示用的 OpenTelemetry Collector、Tempo、Loki、Mimir 与 Grafana。应用通过 OTLP 把 Trace、Metric、Log 发给 Collector，再由 Collector 分发到各存储。

这套 Compose 配置不是生产方案。上线前必须补 TLS、认证、租户隔离、容量、保留期限、备份和升级策略。

## 组件

| 组件 | 用途 | 本地端口 |
| --- | --- | --- |
| OpenTelemetry Collector Contrib | 接收 OTLP 并分发三信号 | 4317 / 4318 |
| Tempo | Trace 存储与查询 | 3200 |
| Loki | Log 存储与查询 | 3100 |
| Mimir | Metric 存储与 Prometheus 兼容查询 | 9009 |
| Grafana | 统一查询与预置面板 | 3000 |

## 启动与停止

从仓库根目录运行：

```powershell
docker compose -f .\observability\docker-compose.yml up -d
docker compose -f .\observability\docker-compose.yml ps
```

```bash
docker compose -f ./observability/docker-compose.yml up -d
docker compose -f ./observability/docker-compose.yml ps
```

停止：

```bash
docker compose -f observability/docker-compose.yml down
```

## 发送示例数据

PowerShell：

```powershell
$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
go run ./example
```

bash：

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example
```

打开 `http://127.0.0.1:3000` 后，可在 Explore 中选择 Tempo、Loki 或 Mimir。默认本地登录信息仅适合开发环境，禁止直接用于公网部署。

## 应用侧装配

公开包 [`telemetry`](../telemetry/) 提供三信号 Provider、资源属性、OTLP gRPC 导出和统一 Shutdown。默认 trace 头部采样率为 `0.1`，批量间隔分别为 trace 5 秒、metric 15 秒、log 1 秒。

Collector 的 `tail_sampling` 只能处理已经到达 Collector 的 trace。SDK 头部采样丢弃的 trace 无法恢复；需要按错误或延迟做尾部决策时，应用侧通常设 `TraceSampleRatio=1.0`，再配置 Collector 采样并完成容量评估。

## 配置入口

- 应用与环境变量：[配置指南](../docs/configuration.md)
- 带注释的应用/Collector 模板：[`example/16_config`](../example/16_config/)
- 可复制的分信号管线：[`templates`](templates/)
- 指标命名建议：[`templates/metric-names.example.md`](templates/metric-names.example.md)

镜像使用固定版本以减少本地环境漂移；升级前应阅读各上游项目的 release notes，并重新验证健康检查、数据源和 dashboard 查询。
