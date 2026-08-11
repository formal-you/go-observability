// Package otelutil 提供框架无关的 OpenTelemetry 集成工具：链路上下文注入/提取与
// log.TraceExtractor 适配。不属于任何框架的中间件，供接入方与框架适配包复用。
package otelutil

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	log "github.com/formal-you/go-observability/log"
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

// InjectHTTPHeaders 把当前 ctx 的传播上下文（traceparent/tracestate）注入 HTTP
// 请求头：调用方在发出下游 HTTP 请求前调用，使下游服务经 Extract 接续本链路。
func InjectHTTPHeaders(ctx context.Context, header http.Header) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(header))
}

// InjectGRPCMetadata 把当前 ctx 的传播上下文注入 gRPC 出站 metadata 并返回新 ctx：
// 调用方在发出下游 gRPC 调用时用返回的 ctx，使下游服务经 Extract 接续本链路。
func InjectGRPCMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	otel.GetTextMapPropagator().Inject(ctx, MetadataCarrier(md))
	return metadata.NewOutgoingContext(ctx, md)
}

// IncomingMetadata 返回 ctx 中 gRPC 入站 metadata 的 TextMapCarrier 适配（无则空）。
func IncomingMetadata(ctx context.Context) MetadataCarrier {
	md, _ := metadata.FromIncomingContext(ctx)
	return MetadataCarrier(md)
}

// MetadataCarrier 把 gRPC metadata 适配为 propagation.TextMapCarrier（键小写）。
type MetadataCarrier metadata.MD

// Get 返回 key 的第一个值。
func (c MetadataCarrier) Get(key string) string {
	values := metadata.MD(c).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Set 设置 key 的值。
func (c MetadataCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

// Keys 返回全部键。
func (c MetadataCarrier) Keys() []string {
	md := metadata.MD(c)
	keys := make([]string, 0, len(md))
	for k := range md {
		keys = append(keys, k)
	}
	return keys
}
