# observability — LGTM 参考栈（Trace / Log / Metric）

本目录是 go-observability 的本地可运行 Observability 参考栈，对应设计决策 A1（LGTM：
Tempo 存储 trace、Loki 存储 log、Mimir 存储 metric、Grafana 展示）与 A2（collector 用
contrib 发行版）。本组件（根包 log 的 writer/otlp 等）统一走 OTLP 直推 collector。

## 组成

- otel-collector（otel/opentelemetry-collector-contrib）— 接收 OTLP（4317/4318），批量后分发
- tempo — trace 存储与查询（3200）
- loki — log 存储与查询（3100，/otlp 接收）
- mimir — metric 存储与查询（9009，Prometheus 兼容面）
- grafana — 统一看板（3000，已预置 tempo/loki/mimir 三个 datasource 与 go-observability-overview 联动面板）

## 快速启动

    docker compose -f observability/docker-compose.yml up -d

停止：

    docker compose -f observability/docker-compose.yml down

## 健康检查

| 组件 | 地址 | 期望 |
| --- | --- | --- |
| Tempo | http://127.0.0.1:3200/ready | 200 |
| Loki | http://127.0.0.1:3100/ready | 200 |
| Mimir | http://127.0.0.1:9009/ready | 200 |
| Grafana | http://127.0.0.1:3000 | 200（admin/admin） |

## 验证

1. 起栈后四个健康地址返回 200；
2. Grafana -> Connections -> Data sources：Tempo / Loki / Mimir 显示连通；
3. 运行 example（设 OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317）并请求后：
   - Grafana -> Explore -> Tempo 查 trace（service.name=go-observability）；
   - Grafana -> Explore -> Loki 查 access/business/error 日志（event.name、error.type 过滤）；
   - Grafana -> Explore -> Mimir 查指标（metric 清单见 B5，直方图桶待定）。

## Go 侧装配（internal/telemetry）

仓库已实现 internal/telemetry：Trace / Metric / Log 三信号 provider + OTLP gRPC 导出，
A3 采样/频率（trace 头部采样默认 0.1、trace 5s、metric 15s、log 1s）与 A7 资源属性
（service.name/version/env + region/instance）。example 已装配。

环境变量：

- OTEL_SDK_DISABLED=true 离线运行（provider 全部 noop）
- OTEL_EXPORTER_OTLP_ENDPOINT（默认 127.0.0.1:4317）
- GO_OBSERVABILITY_REGION / GO_OBSERVABILITY_INSTANCE（A7 低基数标签）
- TraceSampleRatio 在 Config 可调（0-1，默认 0.1；严格 100% 错误 trace 设 1.0）

## Grafana 看板（初版）

已预置 go-observability-overview 面板（Trace / Log / Metric 联动）：

- Tempo 面板：TraceQL 按 service.name 查最近 trace；
- Loki 面板：错误日志（error.type 非空）与访问日志；日志行 trace_id 已配 derived field，点击跳 Tempo（log 到 trace 联动）；
- Mimir 面板：请求错误率 PromQL 示例（semconv 名 http.server.request.duration，B5 定稿后校准命名与直方图桶）；exemplar 已配（metric 到 trace 联动）。

Metric 名与直方图桶以 B5 定稿为准。

## 配置模板（使用者自有）

见 [templates/](templates/)：Log / Trace / Metric 管线与 Error 告警示例、Metric 命名惯例。
库不强制业务指标集；复制后按自己的服务改 endpoint、桶、阈值。

## 待你确定清单

1. Go 侧 Trace/Metric provider 配置 —— 已落地（internal/telemetry，2026-08-09；采样率、资源标签可调）。
2. Grafana 初版面板 —— 已落地（go-observability-overview + datasource 联动）。
3. Metric 命名与直方图桶 —— **已定：使用方自定**（B5；`Providers.Meter()` + templates/ + `example/metrics`）。
4. 是否需要 CI/发布流程（Roadmap / 开源工程）。
5. collector tail_sampling 错误必采的判定属性（templates/trace-pipeline 注释块；接入方定）。

## 备注

- 镜像固定到具体版本 tag（collector 0.158.0 / tempo 2.9.4 / loki 3.6.15 / mimir 3.1.4 / grafana 13.0.6，2026-08-09 确认）。
- 指标走 Prometheus remote-write 兼容面进 Mimir（A1：Prometheus 仅作格式/兼容面，不作长期存储）。
- collector 日志采集默认 OTLP 直推（本组件 writer 已是 OTLP）；filelog 零代码采集见 otel-collector-config.yaml 注释块（参考）。