# example/config — 接入方配置样例（带字段注释）

复制到自己的部署目录后改值。**库不读取这些文件**；由你的进程 / compose / K8s 注入环境变量或挂载。

| 文件 | 用途 |
| --- | --- |
| [`.env.example`](.env.example) | 应用侧环境变量（telemetry / 出口 / 资源标签） |
| [`app.example.yaml`](app.example.yaml) | 应用配置结构示意（字段说明；需自行 load） |
| [`collector.example.yaml`](collector.example.yaml) | OTel Collector 最小管线（Log/Trace/Metric） |
| [`docker-compose.example.yml`](docker-compose.example.yml) | 本地一键：应用 + 链向仓库 `observability/` 栈 |

完整 LGTM 栈见仓库根 [`observability/`](../../observability/)。

告警 / 分信号管线骨架见 [`observability/templates/`](../../observability/templates/)。
