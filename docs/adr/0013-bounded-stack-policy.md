# ADR-0013：有界 StackPolicy 与路径治理

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0002、ADR-0010、ADR-0012

## 背景（Context）

现有 StackPolicy 可以按 ErrorType 选择 must / optional / none，但完整 `debug.Stack` 没有大小上限，并可能暴露构建机绝对路径。仅删除堆栈会损失 panic 的关键诊断信息；把完整堆栈交给日志后端又会造成单条事件失控。

## 决策（Decision）

- `StackConfig` 统一配置最大 UTF-8 字节数、路径策略与 ErrorType 前缀覆盖。配置只应在进程启动阶段设置。
- 最大字节数包含稳定截断标记。截断不得产生非法 UTF-8；日志使用 `app.stacktrace_truncated=true` 区分超限截断，未发生截断时为 false，没有堆栈时不输出该键。
- 路径策略为 `full`、`base`、`redacted`：分别保留完整路径、仅保留文件名、或用固定文本替代路径。该策略同时作用于 stacktrace 与 `code.file.path`。
- 开发默认值为 64 KiB + full；production profile 为 16 KiB + base，并可覆盖高频 ErrorType。
- 严格 `SetStackConfig` 必须保证 `runtime.panic` 仍为 must。panic 堆栈可以截断和裁剪路径，但不能静默变为 none。
- 外部 Error Storage 不是核心依赖。未来接入只能通过注入接口写出，并以稳定 `app.error_id` 关联；本 ADR 不引入对象存储客户端。

## 结果（Consequences）

系统错误堆栈具有确定性大小和路径暴露边界，仍保留现有 must / optional / none 语义。旧 `SetStackPolicy` 继续作为兼容入口；新生产代码应使用可校验的 `SetStackConfig`。
