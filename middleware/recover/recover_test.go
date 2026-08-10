package recovermw

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability"
)

// captureWriter 捕获 Logger 写出的 msg 与扁平 attrs，用于断言中间件发出的事件形状。
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

func attrString(t *testing.T, attrs map[string]any, key, want string) {
	t.Helper()
	v, ok := attrs[key].(slog.Value)
	if !ok {
		t.Errorf("缺少属性 %s（实际: %v）", key, keysOf(attrs))
		return
	}
	if got := v.String(); got != want {
		t.Errorf("%s = %v, want %s", key, got, want)
	}
}

func TestMiddlewareCatchesPanicAndEmitsErrorEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{
		Logger:    logger,
		RequestID: func(*gin.Context) string { return "req-panic-1" },
	}))
	r.GET("/boom", func(c *gin.Context) { panic("boom") })

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应体失败: %v（body=%s）", err, rec.Body.String())
	}
	if resp["code"] != "SYS_ERROR" {
		t.Errorf("code = %v, want SYS_ERROR", resp["code"])
	}
	if resp["request_id"] != "req-panic-1" {
		t.Errorf("request_id = %v, want req-panic-1", resp["request_id"])
	}

	if len(w.msgs) != 1 || w.msgs[0] != "error" {
		t.Fatalf("msgs = %v, want [error]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "error.runtime.panic")
	attrString(t, attrs, "error.type", "runtime.panic")
	attrString(t, attrs, "level", "ERROR")
	attrString(t, attrs, "exception.message", "boom")
	if stack, ok := attrs["exception.stacktrace"].(slog.Value); !ok || stack.String() == "" {
		t.Errorf("exception.stacktrace 应非空，实际 %v", attrs["exception.stacktrace"])
	}
}

// TestMiddlewareNilRecoverPassthrough 验证 recover() 返回 nil 时中间件直接放行：
// handler 正常返回（无 panic）时 defer 中 recover() 为 nil，不写错误事件、不改响应。
// 注意：Go 1.21+ 的 panic(nil) 会转为非 nil 的 *runtime.PanicNilError，
// 因此本仓库 Go 1.21+ 下无法用 panic(nil) 触发 nil 分支，只能以正常返回路径覆盖。
func TestMiddlewareNilRecoverPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200（无 panic 应放行）", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want ok", rec.Body.String())
	}
	if len(w.msgs) != 0 {
		t.Errorf("无 panic 不应写错误事件，实际 %v", w.msgs)
	}
}

func TestMiddlewareFillsTraceAndSpanFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/boom", func(c *gin.Context) { panic("boom") })

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
	req := httptest.NewRequest(http.MethodGet, "/boom", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "trace_id", "0123456789abcdef0123456789abcdef")
	attrString(t, attrs, "span_id", "0123456789abcdef")
}

// TestResponseProjectorOverride 验证 recover 的 ResponseProjector 可注入自定义
// panic 响应形状（状态码与响应体由投影决定），错误事件仍照常写出。
func TestResponseProjectorOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(Middleware(Config{
		Logger: logger,
		ResponseProjector: func(_ error, _ string) (int, gin.H) {
			return http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "internal server error"}}
		},
	}))
	engine.GET("/panic", func(c *gin.Context) { panic("boom") })
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"]["code"] != "internal_error" {
		t.Fatalf("body = %#v, want nested internal_error", body)
	}
	if len(w.msgs) != 1 || w.msgs[0] != "error" {
		t.Fatalf("events = %v, want one error event", w.msgs)
	}
}
