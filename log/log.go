package log

import (
	"context"
	"log/slog"
	"time"
)

// Logger 与 Writer：写出已归一化的事件。
// 批量导出不在此层控制：SDK 侧由 telemetry.Config 的批量导出间隔决定
//（trace 5s / metric 15s / log 1s），Collector 侧由 batch processor 的 timeout /
// send_batch_size 二次凑批。本层每次 Emit 直接调用 Writer，不内置批处理或定时器。

// Writer 接收已归一化并扁平化的语义日志事件。
// eventType（首个参数）当前等于事件的 event_type；attrs 同时包含公共 metadata 与 payload 字段。
// Writer 位于核心治理管线末端，因此看到的字段已经完成采样判定和可选脱敏。
// Logger 不会序列化并发写入：Writer 实现应明确自己的并发语义，面向服务端使用时
// 通常必须允许多个 goroutine 并发调用。Writer 返回的错误只用于诊断输出故障，
// 不会作为业务方法返回值向上传播。
type Writer interface {
	Write(ctx context.Context, eventType string, attrs ...slog.Attr) error
}

// ManagedWriter 在 Writer Seam 上增加幂等资源关闭能力。Runtime 创建的 Writer
// 和 NewMultiWriter 返回该 Interface；只实现 Writer 的既有 Adapter 仍保持兼容。
type ManagedWriter interface {
	Writer
	Close(ctx context.Context) error
}

// ErrorHandler 观察 Writer 写入失败。Logger 不会把该错误返回给业务代码。
// attrs 是本次写入切片的浅副本，ErrorHandler 可以安全调整切片本身；属性中通过
// slog.Any 保存的 map、slice 或指针仍可能与原事件共享底层数据。回调在调用日志的
// goroutine 中同步执行，不应长期阻塞，也不应 panic。
type ErrorHandler func(ctx context.Context, eventType string, attrs []slog.Attr, err error)

// Sampler 采样判定：返回 false 丢弃该事件。
// failed/error/blocked/denied 等高价值结果应强制保留（实现按 app.result 属性判定）。
type Sampler interface {
	Sample(ctx context.Context, attrs []slog.Attr) bool
}

// EventTypeSampler 是可选的类型感知采样扩展。Logger 优先调用 SampleEvent，
// 使采样策略不依赖 EventName 重复携带 access/business/error 等粗分类。
type EventTypeSampler interface {
	SampleEvent(ctx context.Context, eventType EventType, attrs []slog.Attr) bool
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

// IdentityExtractor 从 context.Context 提取已经过认证的 Subject / Actor。
// 非空提取值优先于事件载荷中的同名值。
type IdentityExtractor interface {
	ExtractIdentityContext(ctx context.Context) IdentityContext
}

// IdentityExtractorFunc 允许以函数实现 IdentityExtractor。
type IdentityExtractorFunc func(context.Context) IdentityContext

func (f IdentityExtractorFunc) ExtractIdentityContext(ctx context.Context) IdentityContext {
	return f(ctx)
}

// ContextIdentityExtractor 读取 WithIdentityContext 保存的身份上下文。
type ContextIdentityExtractor struct{}

func (ContextIdentityExtractor) ExtractIdentityContext(ctx context.Context) IdentityContext {
	identity, _ := IdentityContextFromContext(ctx)
	return identity
}

// Option 配置 NewLogger 创建的 Logger。
// Option 只在构造阶段读取。NewLogger 返回后不会再次修改配置，因此 Logger 本身
// 不需要围绕 sampler、masker 或基础 metadata 加锁；这些注入组件仍需自行满足
// Writer、Sampler 和 Masker 各自的并发契约。
type Option func(*loggerOptions)

type loggerOptions struct {
	errorHandler      ErrorHandler
	baseMetadata      EventMetadata
	sampler           Sampler
	masker            Masker
	traceExtractor    TraceExtractor
	identityExtractor IdentityExtractor
	minLevel          Level
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

// WithIdentityExtractor 配置可信身份自动补全。提取出的非空字段优先于事件载荷，
// Actor 字段只注入 SecurityEvent 与 AuditEvent。
func WithIdentityExtractor(extractor IdentityExtractor) Option {
	return func(options *loggerOptions) {
		options.identityExtractor = extractor
	}
}

// WithMinLevel 配置最低写出级别。DEBUG、INFO、WARN、ERROR 依次递增；
// 空值表示不过滤，非法值会在构造 Logger 时 panic。
func WithMinLevel(level Level) Option {
	if level != "" && levelRank(level) < 0 {
		panic("log: 最低日志级别必须是 DEBUG、INFO、WARN 或 ERROR")
	}
	return func(options *loggerOptions) {
		options.minLevel = level
	}
}

// Logger 写出语义日志事件：归一化 attrs 已由事件自身完成，
// 本层顺序执行 默认 metadata/trace 补全 → 最低级别过滤 → 脱敏 → 采样
// → Writer.Write → 错误观察。
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

// Emit 写出一个事件。eventType 为 event_type，attrs 为完整扁平字段。
// 写入失败不会返回给调用方，只交给 ErrorHandler（若有）观察。
func (l *Logger) Emit(ctx context.Context, ev EventPayload) {
	eventType := string(ev.EventType())
	attrs := mergeBaseMetadata(ev.Attrs(), l.options.baseMetadata)
	attrs = ensureTimestamp(attrs)
	if l.options.traceExtractor != nil {
		attrs = fillTraceContext(attrs, l.options.traceExtractor.ExtractTraceContext(ctx))
	}
	if l.options.identityExtractor != nil {
		attrs = applyIdentityContext(attrs, ev.EventType(), l.options.identityExtractor.ExtractIdentityContext(ctx))
	}
	if l.options.minLevel != "" && !atLeastLevel(attrs, l.options.minLevel) {
		return
	}
	if l.options.masker != nil {
		attrs = l.options.masker.Mask(ctx, attrs)
	}
	if l.options.sampler != nil {
		keep := false
		if typed, ok := l.options.sampler.(EventTypeSampler); ok {
			keep = typed.SampleEvent(ctx, ev.EventType(), attrs)
		} else {
			keep = l.options.sampler.Sample(ctx, attrs)
		}
		if !keep {
			return
		}
	}
	if err := l.writer.Write(ctx, eventType, attrs...); err != nil && l.options.errorHandler != nil {
		l.options.errorHandler(ctx, eventType, attrs, err)
	}
}

// applyIdentityContext 用可信非空值替换事件中的同名身份字段。
// 空提取值不删除事件明确提供的字段，便于后台任务显式记录 Subject。
func applyIdentityContext(attrs []slog.Attr, eventType EventType, identity IdentityContext) []slog.Attr {
	replacements := make(map[string]string, 4)
	if identity.Subject.UserID != "" {
		replacements[string(KeyUserID)] = identity.Subject.UserID
		replacements[string(KeyAppUserID)] = identity.Subject.UserID
	}
	if identity.Subject.TenantID != "" {
		replacements[string(KeyAppTenantID)] = identity.Subject.TenantID
	}
	if eventType == EventSecurity || eventType == EventAudit {
		if identity.Actor.UserID != "" {
			replacements[string(KeyAppActorUserID)] = identity.Actor.UserID
		}
		if identity.Actor.Role != "" {
			replacements[string(KeyAppActorRole)] = identity.Actor.Role
		}
	}
	if len(replacements) == 0 {
		return attrs
	}
	out := make([]slog.Attr, 0, len(attrs)+len(replacements))
	for _, attr := range attrs {
		if _, replace := replacements[attr.Key]; replace {
			continue
		}
		out = append(out, attr)
	}
	if userID := identity.Subject.UserID; userID != "" {
		out = append(out, slog.String(string(KeyUserID), userID))
	}
	if tenantID := identity.Subject.TenantID; tenantID != "" {
		out = append(out, slog.String(string(KeyAppTenantID), tenantID))
	}
	if eventType == EventSecurity || eventType == EventAudit {
		if actorID := identity.Actor.UserID; actorID != "" {
			out = append(out, slog.String(string(KeyAppActorUserID), actorID))
		}
		if role := identity.Actor.Role; role != "" {
			out = append(out, slog.String(string(KeyAppActorRole), role))
		}
	}
	return out
}

func atLeastLevel(attrs []slog.Attr, minimum Level) bool {
	for _, attr := range attrs {
		if attr.Key != string(KeyLevel) {
			continue
		}
		rank := levelRank(Level(attr.Value.String()))
		return rank < 0 || rank >= levelRank(minimum)
	}
	// 缺省级别按 INFO 处理，与 OTLP Severity 的缺省行为一致。
	return levelRank(LevelInfo) >= levelRank(minimum)
}

func levelRank(level Level) int {
	switch level {
	case LevelDebug:
		return 0
	case LevelInfo:
		return 1
	case LevelWarn:
		return 2
	case LevelError:
		return 3
	default:
		return -1
	}
}

// hasAttr 判断 attrs 中是否已存在指定键（区分大小写）。
func hasAttr(attrs []slog.Attr, key Key) bool {
	for _, a := range attrs {
		if a.Key == string(key) {
			return true
		}
	}
	return false
}

// ensureTimestamp 为没有显式事件时间的记录补齐当前时间，保证不同 Writer
// 看到同一事件的时间一致，也让 file-only JSONL 每行都可独立排序查询。
func ensureTimestamp(attrs []slog.Attr) []slog.Attr {
	if hasAttr(attrs, KeyTimestamp) {
		return attrs
	}
	return append(attrs, slog.Time(string(KeyTimestamp), time.Now()))
}

// fillTraceContext 仅补全缺失的 trace_id/span_id，不覆盖事件已设置的值。
// 缺失时前置插入，保证 file/stdout 扁平投影里 trace/span 出现在 event.name 之前，
// 符合固定字段顺序（timestamp → level → type → trace/span/request/latency → event.name）。
func fillTraceContext(attrs []slog.Attr, tc TraceContext) []slog.Attr {
	missing := make([]slog.Attr, 0, 2)
	if tc.TraceID != "" && !hasAttr(attrs, KeyTraceID) {
		missing = append(missing, slog.String(string(KeyTraceID), tc.TraceID))
	}
	if tc.SpanID != "" && !hasAttr(attrs, KeySpanID) {
		missing = append(missing, slog.String(string(KeySpanID), tc.SpanID))
	}
	if len(missing) == 0 {
		return attrs
	}
	return append(missing, attrs...)
}

// mergeBaseMetadata 仅补全 attrs 中缺失的公共字段，不覆盖事件已设置的值。
func mergeBaseMetadata(attrs []slog.Attr, base EventMetadata) []slog.Attr {
	if base == (EventMetadata{}) {
		return attrs
	}
	if base.Level != "" && !hasAttr(attrs, KeyLevel) {
		attrs = append(attrs, slog.String(string(KeyLevel), string(base.Level)))
	}
	if base.TraceID != "" && !hasAttr(attrs, KeyTraceID) {
		attrs = append(attrs, slog.String(string(KeyTraceID), base.TraceID))
	}
	if base.SpanID != "" && !hasAttr(attrs, KeySpanID) {
		attrs = append(attrs, slog.String(string(KeySpanID), base.SpanID))
	}
	if base.RequestID != "" && !hasAttr(attrs, KeyRequestID) {
		attrs = append(attrs, slog.String(string(KeyRequestID), base.RequestID))
	}
	if base.LatencyMS != 0 && !hasAttr(attrs, KeyLatencyMS) {
		attrs = append(attrs, slog.Int64(string(KeyLatencyMS), base.LatencyMS))
	}
	return attrs
}
