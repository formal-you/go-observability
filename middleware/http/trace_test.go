package httpmw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceCreatesSpan(t *testing.T) {
	tracer, p := newTestTracer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Trace(TraceConfig{Tracer: tracer})(mux)
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

func TestTracePropagatesSpanContext(t *testing.T) {
	tracer, _ := newTestTracer(t)
	handler := Trace(TraceConfig{Tracer: tracer})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !trace.SpanContextFromContext(r.Context()).IsValid() {
			t.Error("handler 上下文缺少有效 span context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
}

func TestTraceMarks5xxError(t *testing.T) {
	tracer, p := newTestTracer(t)
	handler := Trace(TraceConfig{Tracer: tracer})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

const (
	testTraceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	testTraceID     = "4bf92f3577b34da6a3ce929d0e0e4736"
)

// TestTraceExtractsPropagation 验证入口提取：带 traceparent 的请求，
// 本服务的 server span 接续调用方链路（span parent 的 traceID 与上游一致）。
func TestTraceExtractsPropagation(t *testing.T) {
	tracer, p := newTestTracer(t)
	handler := Trace(TraceConfig{Tracer: tracer})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// TestTraceWithoutPropagationStartsRoot 验证无上游传播时创建新的根 trace。
func TestTraceWithoutPropagationStartsRoot(t *testing.T) {
	tracer, p := newTestTracer(t)
	handler := Trace(TraceConfig{Tracer: tracer})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
