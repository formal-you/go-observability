# ADR-0023：手动分层 span 通用 helper（otelutil.WithSpan / StartSpan）

- 状态：Accepted
- 日期：2026-08-16
- 关联：ADR-0006（中间件分组，otelutil 定位）、CONTEXT.md（Span / 生命周期术语）、docs/architecture.md（采样边界）

## 背景（Context）

Trace 中间件（gin/http/grpc）只为每个请求创建一个 server 根 span（SpanKindServer），
不自动生成 handler→service→store→db 的分层子 span。要形成进程内调用树，接入方必须
在各层手动 `tracer.Start(ctx, ...)` 并传递 ctx。现状痛点：

- 样板代码重复：取 tracer → Start → defer End → 错误 SetStatus → panic 收尾；
- 最容易错且黑盒难发现的是「忘了用 Start 返回的新 ctx」，子 span 静默断链成并列根；
- tracer 来源不统一：各接入方各自持有 tracer，改名/换 provider 时散落。

仓库原则是「能力归库」（AGENTS.md 门禁 4）：这类通用能力应进库，而非每个接入方各写一份。
约束：核心零依赖——`log/` 只依赖标准库与 `errs`；OTel 依赖只允许出现在
`internal/attrkv`、`logwriter/*`、`telemetry`、`middleware/*`、`example/*`。
因此 helper 只能放在已依赖 otel 的 `middleware/otelutil`（框架无关 OTel 工具包），
不能放 `log/`；`middleware/internal/mwutil` 是 internal 包，不对外公开。

## 决策（Decision）

在 `middleware/otelutil` 提供一层薄的手动 span helper，不引入自动埋点。

### 方案 A（主）：WithSpan —— 同步函数包装

```go
// WithSpan 把 fn 包成一个 span：自动 Start/End；fn 返回 err 时
// SetStatus(codes.Error) + RecordError(err)；panic 时标记 Error 后重抛
// （与 mwutil.FinishSpan 语义一致）；返回带新 span 的 ctx 供继续下传。
func WithSpan(ctx context.Context, name string,
	fn func(ctx context.Context) error,
	opts ...SpanOption) (context.Context, error)
```

- `SpanOption` 由本包构造函数提供：`WithTracer(t)` 注入 Tracer（nil→全局
  `otel.Tracer("go-observability")`，对齐 TraceConfig 约定），`WithStartOption(opt)`
  转发 `trace.SpanStartOption`（如 `trace.WithAttributes`、`trace.WithSpanKind`）；
- span name 由调用方给定，是生命周期建模，不是 EventName（不注册，不受 ADR-0018 约束）；
- 错误边界：`WithSpan` 只做 span 信号（SetStatus + RecordError），不代写错误日志事件；
  错误事件仍由 ErrorResponse / Recover 中间件统一收口（ADR-0005 / ADR-0021），
  避免同一错误产生重复日志事件。

### 方案 B（辅）：StartSpan —— 薄封装

```go
// StartSpan 统一 tracer 来源，返回新 ctx + span；生命周期交还调用方，
// 供需要中途写属性/分段结束的场景使用。
func StartSpan(ctx context.Context, name string,
	opts ...SpanOption) (context.Context, trace.Span)
```

## 被否方案

- 方案 C：不加 helper，仅文档化「手动 tracer.Start 模式」：不消除样板与易错点，
  与「能力归库」冲突。
- 方案 D：引入 otel contrib 自动埋点（otelsql / otelhttp client / gorm 插件）：
  新增依赖面大、版本对齐与 CI 成本高，超出本次范围；列为后续独立 ADR 候选。
- 方案 E：做成 AOP/切面框架：过度抽象，掩盖调用方逻辑。

## 结果（Consequences）

- 正面：接入方一行包一层函数即可获得正确嵌套 span；ctx 传播被强制（返回新 ctx）；
  tracer 来源统一；`example/blackbox` 的后台子 span 已改用 helper 作为可验证示例。
- 代价：新增公开 API（v0.x 破坏性变更需在 CHANGELOG 标明并给迁移说明）；
  需要黑盒测试锁定父子关系与错误/panic 收尾；文档（architecture.md / CONTEXT.md /
  README / CHANGELOG）与实现同提交（AGENTS.md 单一真源）。
- 兼容：不影响现有中间件行为与日志关联；子 span 与根 span 同 trace，受 SDK 头部
  采样（TraceSampleRatio）一致控制，不改变现有采样边界。