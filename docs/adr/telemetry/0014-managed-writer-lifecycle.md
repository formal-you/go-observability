# ADR-0014：通过 ManagedWriter 统一 Writer 生命周期

- 状态：Accepted
- 日期：2026-08-12
- 关联：ADR-0011、Issue #19

## 背景（Context）

`log.Writer` 是核心日志包的最小写入 Seam，只要求 `Write`，因此第三方和测试 Adapter 不需要承担资源生命周期。file、stdout 和 OTLP Writer 另一方面拥有文件句柄或 LoggerProvider，需要在进程退出时关闭。

此前 `Runtime.NewWriter` 返回 `log.Writer`，示例和调用方通过匿名 `Close(context.Context)` 类型断言发现关闭能力。这使 Adapter 的生命周期知识跨越 Seam，降低了调用方的 Locality；`MultiWriter` 也只能组合写入，无法统一关闭子 Writer。

## 决策（Decision）

- 保留 `log.Writer` 不变，继续作为兼容且最小的写入 Interface。
- 新增 `log.ManagedWriter`，嵌入 `Writer` 并提供 `Close(context.Context) error`。
- `telemetry.Runtime.NewWriter` 返回 `log.ManagedWriter`。file、stdout、OTLP 和 none 出口统一通过 `log.ManageWriter` 暴露该 Seam。
- `ManageWriter` 对已有 `Close` 的 Adapter 转发关闭；对仅实现 `Write` 的旧 Adapter 提供 no-op 关闭。关闭操作幂等，并在重复调用时返回第一次结果。
- `NewMultiWriter` 返回 `ManagedWriter`。它继续按顺序写入全部子 Writer，并在关闭时尝试所有实现关闭能力的子 Writer，使用 `errors.Join` 聚合错误；不可关闭的旧 Writer 可正常组合。
- Deprecated `Runtime.NewLogWriter` 保持原返回类型和具体 Adapter 行为，避免旧代码的类型断言突然失效。新代码应使用 `Runtime.NewWriter`。
- Runtime 不接管由 `NewWriter` 返回的 Writer；Writer 只关闭自己拥有的资源，复用 Runtime LoggerProvider 的 OTLP Writer 不关闭外部 Provider。

## 被否决的方案

- **直接给 `log.Writer` 增加 Close**：会破坏所有只实现 `Write` 的第三方 Adapter，扩大核心零依赖 Interface 的迁移面。
- **仅新增统一 `CloseWriter(log.Writer)` 辅助函数**：可以减少重复，但调用方仍需接收最小 Writer，生命周期能力不能由 `Runtime.NewWriter` 的 Interface 表达，Leverage 不足。

## 结果（Consequences）

- Runtime 新入口的 Interface 同时表达写入和关闭，示例不再了解具体 Adapter 或执行类型断言。
- 旧 Writer 仍可注入 `Logger`，并可用 `ManageWriter` 获得兼容的托管生命周期。
- `MultiWriter` 的关闭顺序为构造顺序，所有子 Writer 都会被尝试关闭；调用方需要观察聚合关闭错误。
- file/stdout/OTLP Writer 的具体 `Close` 实现仍属于各自 Adapter，核心 `log/` 不引入外部依赖。

## 补充（2026-08-15）

- ADR-0020 删除 `Runtime.NewLogWriter` 兼容层，并将 `Runtime.NewWriter(ctx, WriterConfig)` 改为
  `Runtime.NewWriter(ctx)`；日志文件路径与 options 已并入 `Config.Log`。
- `log.ManagedWriter`、`log.ManageWriter`、`NewMultiWriter` 的关闭与聚合契约保持不变。
