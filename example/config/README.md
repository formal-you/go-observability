# 配置模板

这些文件是可复制的部署参考，库不会自动读取 YAML 或 `.env`。应用或部署平台负责加载和注入。

| 文件 | 用途 |
| --- | --- |
| [`.env.example`](.env.example) | 应用环境变量模板 |
| [`app.example.yaml`](app.example.yaml) | 应用配置结构示意，需要自行解析 |
| [`collector.example.yaml`](collector.example.yaml) | Trace、Metric、Log Collector 管线 |
| [`docker-compose.example.yml`](docker-compose.example.yml) | 应用与参考栈的 Compose 连接示意 |

完整本地栈见 [`observability`](../../observability/)；配置字段与优先级见 [配置指南](../../docs/configuration.md)。复制后务必替换服务名、环境、地址和认证配置，不要把真实凭证提交到 Git。
