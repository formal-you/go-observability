package otelutil

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// captureProcessor 收集 OnEnd 的 span 供断言。
type captureProcessor struct {
	spans []sdktrace.ReadOnlySpan
}

func (p *captureProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}
func (p *captureProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	p.spans = append(p.spans, s)
}
func (p *captureProcessor) Shutdown(context.Context) error   { return nil }
func (p *captureProcessor) ForceFlush(context.Context) error { return nil }

func newTestTracer(t *testing.T) (trace.Tracer, *captureProcessor) {
	t.Helper()
	p := &captureProcessor{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(p), sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() { otel.SetTextMapPropagator(prevProp) })

	return tp.Tracer("go-observability-test"), p
}

// TestInjectHTTPHeaders 验证出口注入：ctx 内 span 的 traceID 出现在 traceparent 头。
func TestInjectHTTPHeaders(t *testing.T) {
	tracer, _ := newTestTracer(t)
	_, span := tracer.Start(context.Background(), "parent")
	header := http.Header{}
	InjectHTTPHeaders(trace.ContextWithSpan(context.Background(), span), header)
	if got := header.Get("traceparent"); !strings.Contains(got, span.SpanContext().TraceID().String()) {
		t.Fatalf("traceparent = %q, want 包含 %s", got, span.SpanContext().TraceID().String())
	}
}

// TestInjectGRPCMetadata 验证 gRPC 出口注入：返回的 ctx 出站 metadata 含 traceparent。
func TestInjectGRPCMetadata(t *testing.T) {
	tracer, _ := newTestTracer(t)
	_, span := tracer.Start(context.Background(), "parent")
	ctx := InjectGRPCMetadata(trace.ContextWithSpan(context.Background(), span))
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("出站 metadata 缺失")
	}
	if got := md.Get("traceparent"); len(got) != 1 || !strings.Contains(got[0], span.SpanContext().TraceID().String()) {
		t.Fatalf("traceparent = %v, want 包含 %s", got, span.SpanContext().TraceID().String())
	}
}

// TestNewTraceExtractor 验证适配器：有有效 span 时返回其 trace_id/span_id，无 span 时返回空。
func TestNewTraceExtractor(t *testing.T) {
	tracer, _ := newTestTracer(t)
	ctx := context.Background()
	_, span := tracer.Start(ctx, "parent")
	spanCtx := trace.ContextWithSpan(ctx, span)

	ext := NewTraceExtractor()
	if tc := ext.ExtractTraceContext(ctx); tc.TraceID != "" || tc.SpanID != "" {
		t.Fatalf("无 span 时 ExtractTraceContext = %+v, want 空", tc)
	}
	tc := ext.ExtractTraceContext(spanCtx)
	if tc.TraceID != span.SpanContext().TraceID().String() {
		t.Fatalf("traceID = %s, want %s", tc.TraceID, span.SpanContext().TraceID().String())
	}
	if tc.SpanID != span.SpanContext().SpanID().String() {
		t.Fatalf("spanID = %s, want %s", tc.SpanID, span.SpanContext().SpanID().String())
	}
}
