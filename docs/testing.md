# 正式测试契约

本文定义 go-observability 如何把需求预期固化为可执行验收。它适用于新增行为、缺陷修复、schema 迁移、中间件行为和 Writer 投影变更。

## 权威顺序

1. 用户已确认的需求、[CONTEXT.md](../CONTEXT.md) 与已接受 ADR 定义预期。
2. 独立黑盒测试把预期变成可重复执行的正式验收。
3. 实现代码与白盒单元测试证明内部实现，但不能反向修改需求预期。

当三者冲突时，先确认或更新契约，再更新黑盒 Oracle，最后修改实现。禁止仅为让现有代码变绿而降低黑盒断言。

## 什么是有效黑盒测试

有效黑盒测试只通过公开 API、真实协议入口或最终输出观察行为，不读取未导出的实现细节：

- `log/blackbox_log_test.go`、`log/event_keep_sampler_blackbox_test.go` 使用外部测试包验证公开日志契约。
- `example/blackbox/blackbox_test.go` 发起真实 Gin 请求、创建真实 OTel span，并验证 JSONL 与内存 OTLP 输出。
- Writer 黑盒按最终 JSONL / LogRecord 形状断言，不复制内部归一化算法。
- 中间件黑盒按请求、响应、事件数量和关联字段断言，不以内部调用顺序充当 Oracle。

外部 Collector、Loki、Tempo 等不可控系统可以使用内存 exporter 或明确的集成测试边界替代；请求生命周期、trace/span 生成、序列化和错误投影应尽量走真实代码路径。

## 变更流程

1. 为行为写明验收来源和可观察结果；复杂变更在提案或 ADR 中记录决策。
2. 先新增或调整独立黑盒测试，并确认它因缺失目标行为而失败。
3. 完成最小实现，再补白盒单元测试覆盖边界、错误分支和内部不变量。
4. 先运行相关测试，再执行 [开发工作流](workflow.md) 的全量门禁。
5. 契约、黑盒、实现、示例和迁移说明在同一逻辑提交中保持一致。

黑盒测试可以在明确的契约变更中更新，但提交必须同时给出需求或 ADR 依据；纯重构不得改变黑盒期望。

## 当前核心验收

- EventName 文法、ErrorType / ErrorCode 分工以 [ADR-0009](adr/0009-event-name-fact-and-error-code.md) 为准。
- event.name 的 `<event>` 必须是注册 Event Type（Event Name Convention）以 [ADR-0018](adr/0018-event-name-convention.md) 为准。
- ErrorCode / ErrorType 严格文法、构造失败和旧入口兼容以 [ADR-0010](adr/0010-strict-error-construction.md) 为准。
- ErrorType 复用 OTel/gRPC 标准枚举（跨模块闭合枚举）以 [ADR-0016](adr/0016-errortype-otel-grpc-standard-enum.md) 为准。
- ErrorCode → ErrorType 固定映射（Error Registry）以 [ADR-0015](adr/0015-error-registry.md) 为准，严格构造器对已注册码强制类型一致。
- file/stdout 与 OTLP 双投影以 [OTel Logs 映射](otel-logs-data-model.md) 为准。
- HTTP AccessEvent 完整性、跨事件关联和后台事件边界以 [`example/blackbox`](../example/blackbox/README.md) 的公开场景与黑盒断言为准；最初的实施计划已归档为 [PR #16 历史提案](history/pr-0016-logging-system-optimization.md)。
- 采样只在使用方显式配置后改变保留行为；默认行为必须由黑盒覆盖。
