package trace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

	// 全局 TextMapPropagator 默认 no-op；模拟生产（telemetry.Setup 安装 W3C TraceContext）。
	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() { otel.SetTextMapPropagator(prevProp) })

	return tp.Tracer("go-observability-test"), p
}

func attrStr(t *testing.T, span sdktrace.ReadOnlySpan, key string) string {
	t.Helper()
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	t.Fatalf("span 缺少属性 %s（实际: %v）", key, span.Attributes())
	return ""
}

func attrInt(t *testing.T, span sdktrace.ReadOnlySpan, key string) int64 {
	t.Helper()
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsInt64()
		}
	}
	t.Fatalf("span 缺少属性 %s（实际: %v）", key, span.Attributes())
	return 0
}

func TestHTTPMiddlewareCreatesSpan(t *testing.T) {
	tracer, p := newTestTracer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := NewHTTPMiddleware(Config{Tracer: tracer})(mux)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	span := p.spans[0]
	if span.Name() != "GET /ok" {
		t.Fatalf("span name = %q, want GET /ok（ServeMux route 重命名）", span.Name())
	}
	if span.SpanKind() != trace.SpanKindServer {
		t.Fatalf("span kind = %v, want Server", span.SpanKind())
	}
	if attrStr(t, span, "http.request.method") != "GET" {
		t.Fatalf("method attr = %s", attrStr(t, span, "http.request.method"))
	}
	if attrInt(t, span, "http.response.status_code") != http.StatusOK {
		t.Fatalf("status attr = %d", attrInt(t, span, "http.response.status_code"))
	}
	if attrStr(t, span, "http.route") != "GET /ok" {
		t.Fatalf("route attr = %s", attrStr(t, span, "http.route"))
	}
	if span.Status().Code == codes.Error {
		t.Fatalf("span status = %v, want 非 Error（成功请求默认 Unset/Ok）", span.Status())
	}
}

func TestHTTPMiddlewarePropagatesSpanContext(t *testing.T) {
	tracer, _ := newTestTracer(t)
	handler := NewHTTPMiddleware(Config{Tracer: tracer})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !trace.SpanContextFromContext(r.Context()).IsValid() {
			t.Error("handler 上下文缺少有效 span context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
}

func TestHTTPMiddlewareMarks5xxError(t *testing.T) {
	tracer, p := newTestTracer(t)
	handler := NewHTTPMiddleware(Config{Tracer: tracer})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	if p.spans[0].Status().Code != codes.Error {
		t.Fatalf("span status = %v, want Error（5xx）", p.spans[0].Status())
	}
}

func TestGinMiddlewareCreatesSpan(t *testing.T) {
	tracer, p := newTestTracer(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(NewGinMiddleware(Config{Tracer: tracer}))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))

	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	span := p.spans[0]
	if span.Name() != "GET /ok" {
		t.Fatalf("span name = %q, want GET /ok", span.Name())
	}
	if attrStr(t, span, "http.route") != "/ok" {
		t.Fatalf("route attr = %s, want /ok", attrStr(t, span, "http.route"))
	}
}

func TestGRPCUnaryInterceptorCreatesSpan(t *testing.T) {
	tracer, p := newTestTracer(t)
	interceptor := NewGRPCUnaryInterceptor(Config{Tracer: tracer})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Register"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}

	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	span := p.spans[0]
	if span.Name() != info.FullMethod {
		t.Fatalf("span name = %q, want FullMethod", span.Name())
	}
	if attrStr(t, span, "rpc.service") != "mall.auth.v1.AuthService" {
		t.Fatalf("rpc.service = %s", attrStr(t, span, "rpc.service"))
	}
	if attrStr(t, span, "rpc.method") != "Register" {
		t.Fatalf("rpc.method = %s", attrStr(t, span, "rpc.method"))
	}
	if attrInt(t, span, "rpc.grpc.status_code") != int64(grpccodes.OK) {
		t.Fatalf("status attr = %d", attrInt(t, span, "rpc.grpc.status_code"))
	}
}

func TestGRPCUnaryInterceptorMarksError(t *testing.T) {
	tracer, p := newTestTracer(t)
	interceptor := NewGRPCUnaryInterceptor(Config{Tracer: tracer})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Login"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(grpccodes.InvalidArgument, "bad")
	})
	if status.Code(err) != grpccodes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	if p.spans[0].Status().Code != codes.Error {
		t.Fatalf("span status = %v, want Error（非 OK）", p.spans[0].Status())
	}
	if attrInt(t, p.spans[0], "rpc.grpc.status_code") != int64(grpccodes.InvalidArgument) {
		t.Fatalf("status attr = %d", attrInt(t, p.spans[0], "rpc.grpc.status_code"))
	}
}

const (
	testTraceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	testTraceID     = "4bf92f3577b34da6a3ce929d0e0e4736"
)

// TestHTTPMiddlewareExtractsPropagation 验证入口提取：带 traceparent 的请求，
// 本服务的 server span 接续调用方链路（span parent 的 traceID 与上游一致）。
func TestHTTPMiddlewareExtractsPropagation(t *testing.T) {
	tracer, p := newTestTracer(t)
	handler := NewHTTPMiddleware(Config{Tracer: tracer})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("traceparent", testTraceParent)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	if got := p.spans[0].Parent().TraceID().String(); got != testTraceID {
		t.Fatalf("span parent traceID = %s, want %s（应接续上游 trace）", got, testTraceID)
	}
}

// TestHTTPMiddlewareWithoutPropagationStartsRoot 验证无上游传播时创建新的根 trace。
func TestHTTPMiddlewareWithoutPropagationStartsRoot(t *testing.T) {
	tracer, p := newTestTracer(t)
	handler := NewHTTPMiddleware(Config{Tracer: tracer})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	if p.spans[0].Parent().IsValid() {
		t.Fatalf("span parent = %v, want 无效（新根 trace）", p.spans[0].Parent())
	}
}

// TestGRPCUnaryInterceptorExtractsPropagation 验证 gRPC 入口提取：metadata 带
// traceparent 时接续调用方链路。
func TestGRPCUnaryInterceptorExtractsPropagation(t *testing.T) {
	tracer, p := newTestTracer(t)
	interceptor := NewGRPCUnaryInterceptor(Config{Tracer: tracer})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Register"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("traceparent", testTraceParent))
	_, err := interceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	if got := p.spans[0].Parent().TraceID().String(); got != testTraceID {
		t.Fatalf("span parent traceID = %s, want %s", got, testTraceID)
	}
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
