# 代码与注释准则

本文是仓库代码实现与 Review 的规范真源。开发步骤、Git 门禁和验证命令仍以
[GitOps SOP](../gitops/gitops-sop.md) 为准；领域词义以 [`CONTEXT.md`](../../CONTEXT.md) 为准。

## 1. Go 代码

- 遵循 Go 官方惯用法，优先使用小而明确的接口、零值可理解的类型和显式错误返回。
- 标识符首字母大写只表示跨包导出，不代表字段应公开。持有资源、锁或生命周期不变量的结构体保持字段私有，通过有业务行为的窄方法暴露能力；避免为每个字段机械添加 Getter / Setter。
- 包边界表达能力所有权：`log/` 保持标准库零依赖，OTel 装配集中在 `telemetry/`，出口实现位于 `logwriter/`。
- 公共 API 以兼容性契约对待。新增能力优先扩展配置或增加独立入口；废弃 API 使用 Go 标准 `Deprecated:` 标记并给出替代入口。
- `context.Context` 作为跨调用取消、截止时间和 Trace Context 的载体，放在参数首位；不存入长期对象，不用 `context.Background()` 覆盖调用方传入的生命周期。
- 错误应包含稳定的包名前缀和操作语义，并使用 `%w` / `errors.Join` 保留错误链。配置错误尽早返回，不能静默忽略不适用的选项。
- 构造完成后尽量保持对象只读。共享可变状态必须明确并发策略，优先使用所有权隔离；计数使用 `atomic`，复合状态使用 `Mutex`，一次性生命周期使用 `sync.Once`。
- 不在 OTel `Logger.Emit`、SDK `BatchProcessor` 或官方 Provider 外层添加无依据的全局锁。SDK 负责其公开并发契约；本库只保护自己拥有的共享状态。
- 资源所有权必须可追踪：每个构造器明确自建和注入资源在成功、失败回滚及关闭阶段分别由谁负责；关闭操作应说明是否 Flush、是否幂等及调用顺序，不能仅凭“注入”推断所有权。

## 2. 注释

- 注释使用中文叙述，保留标准英文术语和标识符，例如 `Runtime`、`Provider`、`Exporter`、`Writer`、`Resource`、`BatchProcessor`、`TraceID`、`SpanID`、`OTLP`。
- 所有导出类型、常量、字段和函数都应有符合 Go Doc 的注释，并以被说明的标识符开头。注释描述契约、边界和可观察行为，不逐字翻译函数名。
- 包注释说明定位和非目标；构造器注释说明创建了什么、是否修改全局状态、资源归谁关闭；并发相关类型说明能否由多个 goroutine 调用。
- 对锁、原子变量、恢复栈、关闭顺序、回滚路径和兼容层写明“保护什么”或“为什么存在”。避免只写“加锁”“设置字段”这类代码复述。
- OTel 注释必须区分 SDK 头部采样（sampling）、SDK 批量导出间隔（export interval）和 Collector 批处理（batch）；不能统称为“采集频率”。
- 注释不得承诺代码没有保证的结果。例如 `Emit` 成功只表示记录交给 SDK，不表示 Collector 或后端已经持久化。
- TODO 必须包含可执行事项，并关联 Issue 或明确移除条件；历史背景和方案取舍进入 ADR，不堆积在源码注释中。

## 3. 设计与实现

- 优先建立深模块：公开接口保持紧凑，校验、默认值、OTel 映射和资源回滚隐藏在所属包内。
- 避免用“改动文件数量”判断内聚性。以职责是否集中、依赖方向是否单向、公共行为是否可独立测试判断模块质量。
- Schema、字段归属和 EventName 是跨出口契约；file/stdout 与 OTLP 可以有不同投影，但必须来自同一归一化事件。
- 热路径避免无界 goroutine、无界队列、反射和重复编码。引入缓存、池化或异步机制前先建立 benchmark 或可复现性能证据。
- 兼容层只做参数映射和委托，不承载新规则；新代码、示例和正式黑盒使用当前 API。

## 4. 测试与 Review

- 公共行为由外部测试包的独立黑盒测试验收；内部白盒测试覆盖校验边界、回滚、并发状态和故障分支。
- 并发类型至少检查多 goroutine 调用、`Write` / `Close` 边界和重复关闭；环境允许时运行 `go test -race ./...`。
- OTel 出口测试分别验证 LogRecord 顶层字段、Attributes、Resource、Trace Context、Flush/Shutdown 和不可用 Collector 的可观察错误。
- Review 注释时同时核对实现：删除失真的注释，补充缺失的所有权与并发契约，不以注释掩盖过深或含混的 API。

## 5. Panic 与 Recover 边界

- panic 只用于启动期（开始接流量前）的配置/契约不变式：`Must*` 注册、`NewEventName`、必填 Logger 为空等；请求路径禁止用 panic 表达业务或系统错误。
- 请求期预期内拒绝走 `errs.BizError`，非预期故障走 `errs.SystemError`，经 `ginmw.Abort` / error return 进入统一错误收口。
- `Recover` 只兜真正的程序缺陷，并写出 `runtime.panic.occurred` + `error.type=INTERNAL` 的 `ErrorEvent`（含 `stacktrace` 与 `code.*`）；它不是容错方案。
- panic 现场诊断以 ErrorEvent 内的 stack/code 为准，pprof 只用于持续剖析，不作为 panic 首要定位工具。
- 决策依据见 [ADR-0021](../adr/errors/0021-panic-boundary-and-recover.md)。

完成标准：修改后的代码通过 `gofmt`、build、vet、全量测试和 `git diff --check`；所有新增公共标识符均有准确 Go Doc，关键并发与资源所有权可仅通过相邻注释判断。
