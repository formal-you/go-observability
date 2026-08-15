# 与 samber slog 生态的关系

[`example/15_samber`](../example/15_samber/) 展示如何把本项目事件属性交给 samber 的 handler 链。它是互操作示例，不代表核心 writer 已由 samber 实现。

| 能力 | 本项目 | samber 生态 |
| --- | --- | --- |
| 类型化可观测事件 | 内置六类事件与字段规范 | 由应用自行定义 |
| JSONL/stdout/OTLP | 提供独立 Writer | 可通过 slog handler 组合，OTLP 需其他桥接 |
| fan-out、格式化、通用采样 | 基础接口，组合能力有限 | `slog-multi`、`slog-formatter`、`slog-sampling` 较成熟 |
| 核心依赖 | log 包只依赖标准库 | 接入相应第三方模块 |

示例验证了 attrs 可以进入 slog 链，但使用时要注意：

- slog 自带字段可能与事件的 level/timestamp 重复，应在适配层去重。
- 基于值模式的 PII formatter 不能替代按业务键维护的脱敏策略。
- samber 的采样与本项目 Logger Sampler 是两个阶段，叠加会改变实际保留率。

当前建议是保留现有 Writer 作为默认路径；需要 fan-out 或复杂 handler 管线的使用方，可以在应用侧增加适配层。引入新的核心依赖应通过兼容性、性能和维护成本评估后再决定。
