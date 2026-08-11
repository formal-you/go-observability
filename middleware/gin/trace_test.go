package ginmw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceCreatesSpan(t *testing.T) {
	tracer, p := newTestTracer(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Trace(TraceConfig{Tracer: tracer}))
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
	if span.SpanKind() != trace.SpanKindServer {
		t.Fatalf("span kind = %v, want Server", span.SpanKind())
	}
	if attrStr(t, span, "http.route") != "/ok" {
		t.Fatalf("route attr = %s, want /ok", attrStr(t, span, "http.route"))
	}
	if span.Status().Code == codes.Error {
		t.Fatalf("span status = %v, want 非 Error（成功请求默认 Unset/Ok）", span.Status())
	}
}
