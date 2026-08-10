package trace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
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
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(p))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
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
