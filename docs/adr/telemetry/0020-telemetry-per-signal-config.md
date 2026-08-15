# ADR-0020：Telemetry 按信号拆分配置并移除兼容层

- 状态：Accepted
- 日期：2026-08-15
- 关联：ADR-0014（取代旧版 Runtime 方案）

## 背景（Context）

旧版 Runtime 引入后，`telemetry.Config` 仍是扁平结构：`LogOutput`、`TraceOutput`、
`MetricOutput` 共享同一个 `LogOutput` 枚举，日志 Writer 参数还拆在 `WriterConfig` 中，
使用方需要在两个地方表达同一个日志出口。旧 `Setup*`、`Providers`、`NewLogWriter`
兼容层让公共表面同时存在两套语义，示例和文档也必须解释“新代码用哪套、旧代码用哪套”。

## 决策（Decision）

- `Config` 拆为 `Resource`、`Trace`、`Metric`、`Log` 四组。
- 用统一 `SignalOutput` 表达信号出口：`file / otlp / stdout / none`；另设仅 Trace 可用的
  `local`，表示只生成合法 TraceID/SpanID、不导出完整 Span。
- `Config.Log` 同时持有输出目标、文件路径、file/stdout options、OTLP 批处理参数；
  `Runtime.NewWriter(ctx)` 不再接收 `WriterConfig`。
- `NewFileRuntime` 保留为公开便捷构造器，内部强制 `Trace=local、Metric=none、Log=file`。
- 删除 `LogOutput`、`Providers`、`WriterConfig`、`Setup`、`SetupFile`、
  `SetupFromEnvironment`、`Runtime.NewLogWriter`、`Runtime.Resource`、
  `Runtime.LoggerProvider`，不再维护源代码兼容层。
- 保留 `EnabledFromEnvironment` / `EndpointFromEnvironment` 作为显式环境变量映射 helper。

## 结果（Consequences）

- 每个信号的输入、默认值和校验集中在对应配置分组中，错误配置在 `NewRuntime` 提前暴露。
- Endpoint 仅在确有信号创建 OTLP exporter/reader 时解析；纯 file/local/stdout/none 组合
  不再受非法 Endpoint 影响。
- `NewFileRuntime` 变成薄的预设构造器，示例不再需要手写三段 local/file/none 配置。
- 公共 API 为破坏性变更：接入方必须迁移 Config 字段和 `NewWriter(ctx)` 调用方式。
- 旧版 Runtime 方案被本 ADR 取代；ADR-0014 的 ManagedWriter 生命周期契约继续有效，仅
  `Runtime.NewWriter` 签名和 `NewLogWriter` 兼容层发生变化。
