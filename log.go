package log

import (
	"context"
	"log/slog"
)

// Logger 与 Writer：写出已归一化的事件。
// 批量导出不在此层控制：SDK 侧由 telemetry.Config 的批量导出间隔决定
//（trace 5s / metric 15s / log 1s），Collector 侧由 batch processor 的 timeout /
// send_batch_size 二次凑批。本层每次 Emit 直接调用 Writer，不内置批处理或定时器。

// Writer 接收已归一化并扁平化的语义日志事件。
// msg 当前等于事件的 event_type；attrs 同时包含公共 metadata 与 payload 字段。
// Writer 位于核心治理管线末端，因此看到的字段已经完成采样判定和可选脱敏。
// Logger 不会序列化并发写入：Writer 实现应明确自己的并发语义，面向服务端使用时
// 通常必须允许多个 goroutine 并发调用。Writer 返回的错误只用于诊断输出故障，
// 不会作为业务方法返回值向上传播。
type Writer interface {
	Write(ctx context.Context, msg string, attrs ...slog.Attr) error
}

// ErrorHandler 观察 Writer 写入失败。Logger 不会把该错误返回给业务代码。
// attrs 是本次写入切片的浅副本，ErrorHandler 可以安全调整切片本身；属性中通过
// slog.Any 保存的 map、slice 或指针仍可能与原事件共享底层数据。回调在调用日志的
// goroutine 中同步执行，不应长期阻塞，也不应 panic。
type ErrorHandler func(ctx context.Context, msg string, attrs []slog.Attr, err error)

// Sampler 采样判定：返回 false 丢弃该事件。
// failed/error/blocked/denied 等高价值结果应强制保留（实现按 app.result 属性判定）。
type Sampler interface {
	Sample(ctx context.Context, attrs []slog.Attr) bool
}

// SamplerFunc 允许以函数实现 Sampler。
type SamplerFunc func(ctx context.Context, attrs []slog.Attr) bool

func (f SamplerFunc) Sample(ctx context.Context, attrs []slog.Attr) bool { return f(ctx, attrs) }

// Masker 脱敏：返回处理后的 attrs（允许返回新切片）。
// 默认实现需递归处理 slog.Any 中的 map 与 []any；自定义敏感类型由调用方显式净化。
type Masker interface {
	Mask(ctx context.Context, attrs []slog.Attr) []slog.Attr
}

// MaskerFunc 允许以函数实现 Masker。
type MaskerFunc func(ctx context.Context, attrs []slog.Attr) []slog.Attr

func (f MaskerFunc) Mask(ctx context.Context, attrs []slog.Attr) []slog.Attr { return f(ctx, attrs) }

// TraceExtractorFunc 允许以函数实现 TraceExtractor。
type TraceExtractorFunc func(ctx context.Context) TraceContext

func (f TraceExtractorFunc) ExtractTraceContext(ctx context.Context) TraceContext { return f(ctx) }

// TraceContext 表示可补充到日志事件中的链路标识。
type TraceContext struct {
	TraceID string
	SpanID  string
}

// TraceExtractor 从 context.Context 中提取链路标识，供 Logger 在事件未显式携带
// trace_id/span_id 时自动补全。适配器由集成层注入（如 middleware/trace.NewTraceExtractor），
// 核心包保持零 OTel 依赖。
type TraceExtractor interface {
	ExtractTraceContext(ctx context.Context) TraceContext
}

// Option 配置 NewLogger 创建的 Logger。
// Option 只在构造阶段读取。NewLogger 返回后不会再次修改配置，因此 Logger 本身
// 不需要围绕 sampler、masker 或基础 metadata 加锁；这些注入组件仍需自行满足
// Writer、Sampler 和 Masker 各自的并发契约。
type Option func(*loggerOptions)

type loggerOptions struct {
	errorHandler   ErrorHandler
	baseMetadata   EventMetadata
	sampler        Sampler
	masker         Masker
	traceExtractor TraceExtractor
}

// WithErrorHandler 配置 Writer 写入失败时的观察函数。
func WithErrorHandler(handler ErrorHandler) Option {
	return func(options *loggerOptions) {
		options.errorHandler = handler
	}
}

// WithBaseMetadata 提供默认 metadata：事件未设置的公共字段（level/trace/span/request/latency）用默认值补全。
func WithBaseMetadata(md EventMetadata) Option {
	return func(options *loggerOptions) {
		options.baseMetadata = md
	}
}

// WithSampler 配置采样判定器。
func WithSampler(sampler Sampler) Option {
	return func(options *loggerOptions) {
		options.sampler = sampler
	}
}

// WithMasker 配置脱敏器。
func WithMasker(masker Masker) Option {
	return func(options *loggerOptions) {
		options.masker = masker
	}
}

// WithTraceExtractor 配置链路标识自动补全：事件未显式设置 trace_id/span_id 时，
// 从 ctx 提取并补全（不覆盖已设置的值）。未配置时保持现状（不补全）。
func WithTraceExtractor(extractor TraceExtractor) Option {
	return func(options *loggerOptions) {
		options.traceExtractor = extractor
	}
}

// Logger 写出语义日志事件：归一化 attrs 已由事件自身完成，
// 本层顺序执行 默认 metadata 补全 → 脱敏 → 采样 → Writer.Write → 错误观察。
type Logger struct {
	writer  Writer
	options loggerOptions
}

// NewLogger 创建 Logger。writer 必填。
func NewLogger(writer Writer, opts ...Option) *Logger {
	if writer == nil {
		panic("log: Writer 不能为空")
	}
	l := &Logger{writer: writer}
	for _, opt := range opts {
		opt(&l.options)
	}
	return l
}

// Emit 写出一个事件。msg 为 event_type，attrs 为完整扁平字段。
// 写入失败不会返回给调用方，只交给 ErrorHandler（若有）观察。
func (l *Logger) Emit(ctx context.Context, ev EventPayload) {
	msg := string(ev.EventType())
	attrs := mergeBaseMetadata(ev.Attrs(), l.options.baseMetadata)
	if l.options.traceExtractor != nil {
		attrs = fillTraceContext(attrs, l.options.traceExtractor.ExtractTraceContext(ctx))
	}
	if l.options.masker != nil {
		attrs = l.options.masker.Mask(ctx, attrs)
	}
	if l.options.sampler != nil && !l.options.sampler.Sample(ctx, attrs) {
		return
	}
	if err := l.writer.Write(ctx, msg, attrs...); err != nil && l.options.errorHandler != nil {
		l.options.errorHandler(ctx, msg, attrs, err)
	}
}

// fillTraceContext 仅补全缺失的 trace_id/span_id，不覆盖事件已设置的值。
func fillTraceContext(attrs []slog.Attr, tc TraceContext) []slog.Attr {
	have := make(map[string]bool, len(attrs))
	for _, a := range attrs {
		have[a.Key] = true
	}
	if tc.TraceID != "" && !have[string(KeyTraceID)] {
		attrs = append(attrs, slog.String(string(KeyTraceID), tc.TraceID))
	}
	if tc.SpanID != "" && !have[string(KeySpanID)] {
		attrs = append(attrs, slog.String(string(KeySpanID), tc.SpanID))
	}
	return attrs
}

// mergeBaseMetadata 仅补全 attrs 中缺失的公共字段，不覆盖事件已设置的值。
func mergeBaseMetadata(attrs []slog.Attr, base EventMetadata) []slog.Attr {
	if base == (EventMetadata{}) {
		return attrs
	}
	have := make(map[string]bool, len(attrs))
	for _, a := range attrs {
		have[a.Key] = true
	}
	if !have[string(KeyLevel)] && base.Level != "" {
		attrs = append(attrs, slog.String(string(KeyLevel), string(base.Level)))
	}
	if !have[string(KeyTraceID)] && base.TraceID != "" {
		attrs = append(attrs, slog.String(string(KeyTraceID), base.TraceID))
	}
	if !have[string(KeySpanID)] && base.SpanID != "" {
		attrs = append(attrs, slog.String(string(KeySpanID), base.SpanID))
	}
	if !have[string(KeyRequestID)] && base.RequestID != "" {
		attrs = append(attrs, slog.String(string(KeyRequestID), base.RequestID))
	}
	if !have[string(KeyLatencyMS)] && base.LatencyMS != 0 {
		attrs = append(attrs, slog.Int64(string(KeyLatencyMS), base.LatencyMS))
	}
	return attrs
}
