# 生态对比：samber slog 系列与 go-observability

> 日期：2026-08-08。结论：samber 覆盖了我们组件的绝大多数零件，但没有「类型化领域事件 + semconv 语义治理 + 双投影」这一层——那正是我们的差异化。

## 对应关系

| samber 仓库 | 定位 | 对应我们 |
| --- | --- | --- |
| samber/slog-multi | slog handler 工作流：fanout / 中间件 / 路由 / failover | Logger 链 / fanout / Writer 抽象 |
| samber/slog-gin | Gin 中间件 | middleware/ginlog |
| samber/slog-formatter | 属性格式化 + PII 匿名化 | Masker |
| samber/slog-sampling | 采样 / 降噪 / rate-limiting | Sampler |
| samber/slog-loki、slog-quickwit、slog-fluentd、slog-parquet、slog-zerolog、slog-zap、slog-webhook、slog-channel、slog-mattermost、slog-microsoft-teams、slog-rollbar | 各后端 handler | writer/otlp、writer/stdout、writer/file |
| samber/slog-echo、slog-fiber、slog-http、slog-chi | 其他框架中间件 | （未实现） |
| samber/slog-otel | OTEL toolchain（很小） | 参考 |
| samber/slog-mock | 测试 mock handler | 测试辅助 |

## samber 没有的（我们的差异化）

- 没有 slog-otlp：OTLP 直出不在 samber，官方路径是 go.opentelemetry.io/contrib/bridges/otelslog。
- 没有类型化事件模型：samber 全是 slog 原生 key-value record；没有六域（access/business/error/audit/security/probe）、没有 EventPayload 接口、没有具体事件结构体、没有中间件类型断言。
- 没有 semconv 语义治理：键不强制对齐 semconv；没有「源即规范 + 双投影（运维面 semconv / 运营面扁平宽表）」。
- 没有设计文档绑定：wayfinder 决策树 / DESIGN-DECISIONS / CONTEXT 词汇表。

## 借鉴路线与边界

- 边界：核心包保持零外部依赖（仅 log/slog、net、time）不变；samber 只进 writer / 中间件 / example 层。
- 借鉴：slog-multi（fanout/failover）、slog-formatter（PII 掩码）、slog-sampling（采样策略，参考「高价值结果保留」思路）、slog-gin（中间件细节对照）。
- 兼容性：我们的 EventPayload.Attrs() 输出 []slog.Attr，与 samber 的 slog handler 生态天然兼容，可无痛接入。
- 实践：feat/samber-integration 分支做 spike 对照实验（example/samber），验证后再决定是否替换 writer 层。

## 决策建议

语义层（类型化事件 + semconv + 双投影）保留；零件层可视验证结果直接依赖或借鉴 samber，减少重复造轮子。