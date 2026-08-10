package trace

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	log "github.com/formal-you/go-observability"
)

// NewTraceExtractor 返回从 ctx 提取当前 span 链路标识的 log.TraceExtractor 适配器。
// 与 log.WithTraceExtractor 搭配：事件未显式携带 trace_id/span_id 时自动补全，
// 使 file/stdout 等扁平投影与 OTLP 一致关联链路；ctx 无有效 span 时返回空值（不补全）。
func NewTraceExtractor() log.TraceExtractor {
	return traceExtractor{}
}

// traceExtractor 实现 log.TraceExtractor：读取 ctx 中当前 span 的 SpanContext。
type traceExtractor struct{}

// ExtractTraceContext 返回 ctx 当前 span 的 trace_id/span_id；无有效 span 时返回空 TraceContext。
func (traceExtractor) ExtractTraceContext(ctx context.Context) log.TraceContext {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return log.TraceContext{}
	}
	return log.TraceContext{TraceID: sc.TraceID().String(), SpanID: sc.SpanID().String()}
}
