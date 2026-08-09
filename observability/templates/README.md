# 配置文件模板（使用者自有）

本目录只提供 **可复制改写的参考模板**，不绑定具体业务指标集。

**带完整字段注释** 的应用侧样例见 [`example/config/`](../../example/config/)。

| 信号 | 模板 | 组件侧状态 | 谁定清单 |
| --- | --- | --- | --- |
| Log | `log-pipeline.example.yaml` | SDK + 事件模型已完备 | 应用决定埋哪些事件 |
| Trace | `trace-pipeline.example.yaml` | SDK 装配 + 采样钩子已完备 | 应用/运维定采样率与 tail 规则 |
| Error | `error-alerts.example.yaml`（+ log/trace） | errs + LevelOf + 投影已完备 | 应用定 error.type 扩展与告警路由 |
| Metric | `metric-pipeline.example.yaml` + `metric-names.example.md` | **库不内置业务指标** | **完全由使用者定义** |

原则：

1. go-observability 提供语义事件、错误模型、OTLP 出口与 LGTM 参考栈骨架。
2. Metric 命名、直方图桶、标签基数、告警阈值一律由接入方决定。
3. 复制到自己的 `deploy/` / `ops/` 后改 `service.name`、endpoint、保留期与阈值。
4. 字段级说明优先读模板内注释与 [docs/configuration.md](../../docs/configuration.md)。
