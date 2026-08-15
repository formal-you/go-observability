package ginmw

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/errs"

	"github.com/formal-you/go-observability/log"
)

func TestRecoverCatchesPanicAndEmitsErrorEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Recover(RecoverConfig{
		Logger:       logger,
		GetRequestID: func(*gin.Context) string { return "req-panic-1" },
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

	if len(w.eventTypes) != 1 || w.eventTypes[0] != "error" {
		t.Fatalf("eventTypes = %v, want [error]", w.eventTypes)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "runtime.panic.occurred")
	attrString(t, attrs, "error.type", "INTERNAL")
	attrString(t, attrs, "level", "ERROR")
	attrString(t, attrs, "exception.message", "boom")
	if stack, ok := attrs["exception.stacktrace"].(slog.Value); !ok || stack.String() == "" {
		t.Errorf("exception.stacktrace 应非空，实际 %v", attrs["exception.stacktrace"])
	}
}

// TestRecoverNilPassthrough 验证 recover() 返回 nil 时中间件直接放行：
// handler 正常返回（无 panic）时 defer 中 recover() 为 nil，不写错误事件、不改响应。
func TestRecoverNilPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Recover(RecoverConfig{Logger: logger}))
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
	if len(w.eventTypes) != 0 {
		t.Errorf("无 panic 不应写错误事件，实际 %v", w.eventTypes)
	}
}

func TestRecoverFillsTraceAndSpanFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Recover(RecoverConfig{Logger: logger}))
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

// TestRecoverProjectorOverride 验证 Recover 的 ResponseProjector 可注入自定义
// panic 响应形状（状态码与响应体由投影决定），错误事件仍照常写出。
func TestRecoverProjectorOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(Recover(RecoverConfig{
		Logger: logger,
		ResponseProjector: func(_ error, _ string) (int, any) {
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
	if body["error"]["code"] != "internal_error" || body["error"]["message"] == "" {
		t.Fatalf("body = %#v, want nested internal_error", body)
	}
	if len(w.eventTypes) != 1 || w.eventTypes[0] != "error" {
		t.Fatalf("events = %v, want one error event", w.eventTypes)
	}
}

func TestRecoverPanicsOnInvalidEventName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid Recover EventName")
		}
	}()
	logger := log.NewLogger(&captureWriter{})
	Recover(RecoverConfig{Logger: logger, EventName: log.EventName("bad name")})
}

func TestRecoverPanicsOnInvalidErrorType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid Recover ErrorType")
		}
	}()
	logger := log.NewLogger(&captureWriter{})
	Recover(RecoverConfig{Logger: logger, ErrorType: errs.ErrorType("bad.type")})
}
