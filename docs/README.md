# 文档索引

| 文档 | 适合何时阅读 |
| --- | --- |
| [CONTEXT.md](../CONTEXT.md) | 统一项目术语，避免采样/批量导出间隔/事件模型等概念沟通误导 |
| [onboarding.md](onboarding.md) | 首次克隆后快速运行与定位代码 |
| [configuration.md](configuration.md) | 配置 telemetry、Logger 和部署侧 Collector |
| [environment.md](environment.md) | 查询库实际读取的环境变量 |
| [architecture.md](architecture.md) | 理解包边界和事件数据流 |
| [coding-standards.md](coding-standards.md) | 编写或 Review Go 实现、公共 API、并发代码和库级注释 |
| [adr/](adr/README.md) | 追溯关键架构决策（ADR）：错误模型等「为什么这么做」的记录 |
| [otel-logs-data-model.md](otel-logs-data-model.md) | OTel Logs LogRecord 各顶层字段与 go-observability 的映射 |
| [testing.md](testing.md) | 需求、缺陷或公共行为变更时，如何用独立黑盒测试固化正式验收 |
| [security.md](security.md) | 设计脱敏、采样和日志治理策略 |
| [gitops/](gitops/README.md) | GitOps 治理组：GitOps SOP、分支命名、Issue/PR 记录与归档、远程实操、发布清单 |
| [dev-sop.md](dev-sop.md) | 开发任务 SOP（demo 草案）：需求收敛、多 Agent 编排、契约先行到提交的完整剧本 |
| [samber-comparison.md](samber-comparison.md) | 评估与 samber slog 生态的取舍 |
| [todo.md](todo.md) | 非代码待办与飞书项目管理迁移准备 |
| [reports/](reports/README.md) | 查看 Issue 验收、性能和 mutation 基线证据 |
| [go-observability-architecture.drawio](go-observability-architecture.drawio) | 可编辑架构图 |
| [module-map.html](module-map.html) | 浏览器直接打开的 UML 模块地图 |
| [blog/](blog/README.md) | 面向掘金的体系化文章源稿 |

可复制的部署配置位于 [`example/16_config`](../example/16_config/)；本地 LGTM 栈位于 [`observability`](../observability/)；分信号模板位于 [`observability/templates`](../observability/templates/)。

开源协作文档：[贡献指南](../CONTRIBUTING.md)、[贡献者公约](../CODE_OF_CONDUCT.md)、[安全政策](../SECURITY.md)、[支持渠道](../SUPPORT.md)、[变更记录](../CHANGELOG.md)。
