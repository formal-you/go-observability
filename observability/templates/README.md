# 管线配置模板

本目录提供可复制的参考配置，不是库的运行时配置，也不承诺适合生产默认值。

| 信号 | 文件 | 使用方需要决定 |
| --- | --- | --- |
| Log | [`log-pipeline.example.yaml`](log-pipeline.example.yaml) | 脱敏、保留期、租户、索引与访问控制 |
| Trace | [`trace-pipeline.example.yaml`](trace-pipeline.example.yaml) | SDK 头部采样率、Collector 尾部规则与容量 |
| Metric | [`metric-pipeline.example.yaml`](metric-pipeline.example.yaml) | 指标名、单位、直方图桶与标签基数 |
| Error | [`error-alerts.example.yaml`](error-alerts.example.yaml) | `error.type`、阈值、窗口与通知路由 |

指标命名建议见 [`metric-names.example.md`](metric-names.example.md)。复制到应用的部署目录后，至少修改 endpoint、`service.name`、认证、保留期限和告警阈值。

重要采样限制：应用 SDK 以 `TraceSampleRatio=0.1` 做头部采样时，未导出的 trace 不会到达 Collector，`tail_sampling` 无法恢复。要让 Collector 按完整 trace 判断错误或慢请求，应用侧通常需要先设为 `1.0`。
