# 统一 Writer 生命周期

- 状态：实施中
- 远程 Issue：[#19](https://github.com/formal-you/go-observability/issues/19)
- 优先级：P1
- 架构建议强度：Strong
- 创建日期：2026-08-12

## 用户问题

`telemetry.Runtime.NewLogWriter` 原先返回 `log.Writer`，该 Interface 只描述写入，不描述资源关闭。file、stdout 和 OTLP Adapter 实际都持有需要关闭的资源，但示例与测试只能重复执行 `Close(context.Context)` 类型断言。

这让 Adapter 的生命周期知识越过 Writer Seam 泄漏到调用方。新增接入点容易忘记关闭，`MultiWriter` 也没有统一表达子 Writer 的所有权与关闭规则。

## 目标

- 让调用方通过公开 Interface 创建、使用并释放 Runtime 创建的 Writer，无需导入具体 Adapter 或执行匿名类型断言。
- 把 file、stdout、OTLP、none 和组合 Writer 的生命周期规则集中在库内，提高 Locality。
- 保持 `log.Writer` 作为最小写入 Seam，使现有自定义 Adapter 继续兼容。
- 通过与调用方相同的公开 Interface 完成正式黑盒验收。

## 范围

- Runtime 创建的 file、stdout、OTLP 和 none Writer 的统一关闭入口。
- Writer 与 Runtime / 外部 LoggerProvider 的资源所有权规则。
- `MultiWriter` 对可关闭与不可关闭子 Writer 的组合规则。
- 示例、配置说明、正式黑盒测试和迁移说明同步更新。
- 在 ADR 中比较候选 Interface，并记录最终选择及兼容性理由。

## 非范围

- 不重构 `telemetry.Config`。
- 不统一 `Sampler` / `EventTypeSampler`。
- 不改变日志字段、事件语义、采样行为或双投影格式。
- 不增加异步 file Writer，也不改变 OTel SDK 批量导出策略。
- 不因测试便利向核心 Interface 暴露 Provider 等内部 Seam。

## 设计约束

1. `log/` 继续只依赖 Go 标准库。
2. 不直接给现有 `log.Writer` 增加强制 `Close` 方法，以免破坏已有自定义 Adapter。
3. Writer 只关闭自己拥有的资源；注入的外部 Provider 继续由其所有者关闭。
4. 关闭错误必须可观察；组合关闭不能因一个子 Writer 失败而跳过其余子 Writer。
5. 新 Interface 应增加调用方 Leverage，而不是要求调用方理解更多具体 Adapter 类型。

## 待决策

实施前至少比较以下方案，并通过 ADR 选择：

- 新增托管 Writer Interface，由 `Runtime.NewLogWriter` 返回写入与关闭能力。（已选择）
- 保持 `Runtime.NewLogWriter` 返回 `log.Writer`，新增统一关闭函数并在内部识别生命周期能力。

比较维度：向后兼容、Interface 大小、组合 Writer 所有权、关闭错误传播、测试是否只穿过公开 Seam。

## 可观察验收

- [x] 调用方可通过公开 Interface 写入并关闭 Runtime 创建的 file、stdout、OTLP、none Writer。
- [x] 仓库生产示例不再包含 `interface { Close(context.Context) error }` 类型断言。
- [x] 现有仅实现 `Write` 的自定义 `log.Writer` 无需修改即可继续编译和使用。
- [x] file Writer 关闭后数据已经写入并可从 JSONL 读取。
- [x] stdout Writer 和自建 OTLP Writer 关闭其拥有的 Provider；复用 Runtime Provider 的 OTLP Writer 不越权关闭 Provider。
- [x] 组合 Writer 尝试关闭全部可关闭子 Writer，并聚合返回关闭错误；仅实现 `Write` 的子 Writer 可正常组合。
- [x] 重复关闭行为有明确文档和测试，不依赖具体 Adapter 的偶然实现。
- [x] 现有黑盒日志语义、字段顺序、AccessEvent 完整性和 OTLP 映射保持不变。

## 测试要求

- 外部测试包只通过公开 Interface 验证创建、写入、关闭和错误传播。
- 使用可控的本地 Adapter 或内存 OTel Provider 验证所有权，不依赖真实 Collector。
- 保留必要的白盒测试验证关闭一次、错误聚合和并发内部不变量，但白盒测试不得代替黑盒验收。
- 完成后执行 `docs/workflow.md` 的完整 Go 门禁和 `git diff --check`。

## 后续候选

以下候选来自同次架构评估，但应建立独立 Issue，避免扩大本需求：

- 收窄 `telemetry.Config` 的三信号配置知识面。
- 在后续破坏性版本统一事件感知的采样 Interface。
