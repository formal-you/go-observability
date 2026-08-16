# 参考与实现真源（Reference）

本组收纳描述「项目实际状况」的参考文档：代码怎么分层、怎么写、怎么配置、读哪些环境变量、OTel 怎么映射。它们是被实现与既有决策约束的真源，不是流程说明。

> 状态：draft 搭建中。定位：实现真源与参考指南。

## 文件路由

| 文档 | 何时读 |
| --- | --- |
| [architecture.md](architecture.md) | 理解包边界、事件数据流、资源所有权与 OTel 边界 |
| [coding-standards.md](coding-standards.md) | 编写或 Review Go 实现、公共 API、并发与注释 |
| [configuration.md](configuration.md) | 配置 telemetry、Logger 和部署侧 Collector |
| [environment.md](environment.md) | 查询库实际读取的环境变量 |
| [otel-logs-data-model.md](otel-logs-data-model.md) | OTel Logs LogRecord 顶层字段与双投影映射 |

## 与其它组的分工

| 文档 | 负责 |
| --- | --- |
| 本组（reference/） | 项目现状与实现真源：代码 / 配置 / 环境变量 / OTel 映射长什么样 |
| [GitOps 治理](../gitops/README.md) | 分支 / Issue / PR / 发布怎么走 |
| [开发流程体系](../dev-sop/README.md) | 一个开发任务怎么正确完成（需求→验收→提交） |

## 未归组（仍留在 docs/ 根）

`onboarding.md`（入门）、`testing.md`（验收契约）、`security.md`（安全指南）、`samber-comparison.md`（评估）——后续如需可再归组。