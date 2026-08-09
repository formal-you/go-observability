# 架构与数据流

## 分层

| 层 | 包 | 职责 |
| --- | --- | --- |
| 事件核心 | 根包 `log` | 事件类型、字段归一化、Logger、采样与脱敏接口 |
| 错误模型 | `errs` | 错误分类、堆栈规则与错误事件投影 |
| 写出 | `writer/file`、`writer/stdout`、`writer/otlp` | 把归一化事件写到不同后端 |
| OTel 映射 | `internal/attrkv` | 在 `slog.Attr` 与 OTel 值之间转换 |
| 三信号装配 | `telemetry` | 创建并关闭 Trace、Metric、Log Provider |
| HTTP 集成 | `middleware/ginlog`、`middleware/recover` | Gin access 日志和 panic 恢复 |

核心事件模型只依赖标准库；OTel SDK 依赖集中在 writer 与 telemetry 层。

## 一条事件如何写出

```text
应用构造类型化事件
  -> Logger 补全基础 metadata
  -> Masker 脱敏
  -> Sampler 判定是否保留
  -> Writer 写 JSONL、stdout 或 OTLP
  -> 写入失败交给 ErrorHandler
```

事件属性保持扁平，便于日志查询和导出。`event.name` 使用 `类别.模块.操作` 三段式；跨领域通用键在核心包维护，领域专属 `business.*` 事件和属性由使用方注册。

OTLP Writer 将 timestamp、severity、EventName 和 span context 映射到 LogRecord 顶层；JSONL 与 stdout 保留便于直接检索的扁平字段。

## Trace、日志与指标

- trace/span 描述一次请求及其中的操作；日志可携带 trace_id 与 span_id 供关联查询。
- 指标语义属于使用方，`telemetry.Providers.Meter` 只提供 Meter 入口。
- `telemetry.SetupFromEnvironment` 在 Setup 时固化 Provider 与日志出口；`Providers.NewLogWriter` 复用该决策选择 OTLP 或 JSONL。
- `Providers.Shutdown` 应在进程退出前调用，确保尽量 flush 三信号。

## 采样边界

默认 `TraceSampleRatio=0.1` 是 SDK 头部采样：未选中的 trace 不会被导出，Collector 看不到，也无法通过 `tail_sampling` 恢复。需要 Collector 按错误或延迟决定保留时，应让 SDK 导出完整 trace（通常为 `1.0`），再在 Collector 执行尾部采样，并评估吞吐、费用和敏感数据风险。

## 扩展业务事件

1. 在业务自己的 observability 包中声明 `EventName` 常量，并调用 `Validate` 测试格式。
2. 使用 `BusinessPayload.ExtraAttrs` 注入领域属性，避免把领域字段加入核心注册表。
3. 添加外部包黑盒测试，验证字段名、结果值、脱敏和 Writer 输出。
4. 公共 schema 或 API 改动同步 README、示例和 CHANGELOG。

完整示例见 [`example/mall`](../example/mall/)；可编辑架构图见 [go-observability-architecture.drawio](go-observability-architecture.drawio)。
