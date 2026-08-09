package ginlog

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability"
)

type captureWriter struct {
	msgs      []string
	attrsList [][]slog.Attr
}

func (w *captureWriter) Write(_ context.Context, msg string, attrs ...slog.Attr) error {
	w.msgs = append(w.msgs, msg)
	w.attrsList = append(w.attrsList, attrs)
	return nil
}

func attrMap(attrs []slog.Attr) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestMiddlewareWritesAccessEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/api/v1/products/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/42", nil)
	req.Header.Set("X-Request-ID", "req-abc")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if len(w.msgs) != 1 || w.msgs[0] != "access" {
		t.Fatalf("msgs = %v, want [access]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	want := []string{
		"event.name", "http.request.method", "http.route", "url.path",
		"http.response.status_code", "request_id", "mall.result",
	}
	for _, k := range want {
		if _, ok := attrs[k]; !ok {
			t.Errorf("缺少属性 %s（实际: %v）", k, keysOf(attrs))
		}
	}
	if got, ok := attrs["http.route"].(slog.Value); !ok || got.String() != "/api/v1/products/:id" {
		t.Errorf("http.route = %v, want 路由模板", got)
	}
}

func TestMiddlewareSkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger, SkipPaths: map[string]bool{"/healthz": true}}))
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if len(w.msgs) != 0 {
		t.Errorf("跳过路径不应写日志，实际 %v", w.msgs)
	}
}

func TestMiddlewareStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/boom", func(c *gin.Context) { c.AbortWithStatus(http.StatusInternalServerError) })

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	attrs := attrMap(w.attrsList[0])
	if got, ok := attrs["level"].(slog.Value); !ok || got.String() != "ERROR" {
		t.Errorf("500 应映射 level=ERROR，实际 %v", attrs["level"])
	}
	if got, ok := attrs["mall.result"].(slog.Value); !ok || got.String() != "failed" {
		t.Errorf("500 应映射 result=failed，实际 %v", attrs["mall.result"])
	}
}

func TestMiddlewareTraceFromOTelSpan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	tid, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	sid, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), sc)
	req := httptest.NewRequest(http.MethodGet, "/ping", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	attrs := attrMap(w.attrsList[0])
	if got, ok := attrs["trace_id"].(slog.Value); !ok || got.String() != "0123456789abcdef0123456789abcdef" {
		t.Errorf("trace_id = %v, want 0123456789abcdef0123456789abcdef", attrs["trace_id"])
	}
	if got, ok := attrs["span_id"].(slog.Value); !ok || got.String() != "0123456789abcdef" {
		t.Errorf("span_id = %v, want 0123456789abcdef", attrs["span_id"])
	}
}

// TestDefaultLevelForStatus 验证 access 状态码→级别映射（B3 Q4 定稿）：
// 2xx-3xx=INFO、4xx=WARN、503=WARN（暂时不可用、调用方可重试）、其余 5xx=ERROR。
func TestDefaultLevelForStatus(t *testing.T) {
	cases := []struct {
		status int
		want   log.Level
	}{
		{http.StatusOK, log.LevelInfo},
		{http.StatusMovedPermanently, log.LevelInfo},
		{http.StatusBadRequest, log.LevelWarn},
		{http.StatusNotFound, log.LevelWarn},
		{http.StatusServiceUnavailable, log.LevelWarn},
		{http.StatusInternalServerError, log.LevelError},
		{http.StatusBadGateway, log.LevelError},
	}
	for _, tc := range cases {
		if got := defaultLevelForStatus(tc.status); got != tc.want {
			t.Errorf("defaultLevelForStatus(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

// TestMiddlewareStatus503IsWarn 端到端验证 503 请求的 access 事件 level=WARN、
// result=failed（B3 Q4：503 记 WARN 而非 ERROR，避免重试噪音淹没告警）。
func TestMiddlewareStatus503IsWarn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/unavailable", func(c *gin.Context) { c.AbortWithStatus(http.StatusServiceUnavailable) })

	req := httptest.NewRequest(http.MethodGet, "/unavailable", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	attrs := attrMap(w.attrsList[0])
	if got, ok := attrs["level"].(slog.Value); !ok || got.String() != "WARN" {
		t.Errorf("503 应映射 level=WARN（B3 Q4），实际 %v", attrs["level"])
	}
	if got, ok := attrs["mall.result"].(slog.Value); !ok || got.String() != "failed" {
		t.Errorf("503 应映射 result=failed，实际 %v", attrs["mall.result"])
	}
}
