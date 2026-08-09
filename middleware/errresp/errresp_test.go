package errresp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/errs"
	recovermw "github.com/formal-you/go-observability/middleware/recover"
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

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func doRequest(r *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestMiddlewareBusinessErrorWritesEventAndResponse 验证业务拒绝（KindBusiness）：
// 状态码 409、响应体透传业务码与消息、事件投影为 business 事件（WARN / failed）。
func TestMiddlewareBusinessErrorWritesEventAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{
		Logger:       logger,
		GetRequestID: func(*gin.Context) string { return "req-biz-1" },
	}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewBusiness(
			"ORDER.CREATE.STOCK_INSUFFICIENT",
			"business.stock_insufficient",
			"stock insufficient",
		))
	})

	rec := doRequest(r, "/orders/1")

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应体失败: %v（body=%s）", err, rec.Body.String())
	}
	if resp["code"] != "ORDER.CREATE.STOCK_INSUFFICIENT" {
		t.Errorf("code = %v, want 业务错误码", resp["code"])
	}
	if resp["message"] != "stock insufficient" {
		t.Errorf("message = %v, want stock insufficient（预期内拒绝应透传）", resp["message"])
	}
	if resp["request_id"] != "req-biz-1" {
		t.Errorf("request_id = %v, want req-biz-1", resp["request_id"])
	}

	if len(w.msgs) != 1 || w.msgs[0] != "business" {
		t.Fatalf("msgs = %v, want [business]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "error.http.request")
	attrString(t, attrs, "error.type", "business.stock_insufficient")
	attrString(t, attrs, "app.business_code", "ORDER.CREATE.STOCK_INSUFFICIENT")
	attrString(t, attrs, "app.business_message", "stock insufficient")
	attrString(t, attrs, "app.result", "failed")
	attrString(t, attrs, "level", "WARN")
	attrString(t, attrs, "request_id", "req-biz-1")
}

// TestMiddlewareValidationErrorWritesEventAndResponse 验证参数校验（KindValidation）：
// 状态码 400、响应体 VALIDATION_ERROR、事件投影为 business 事件。
func TestMiddlewareValidationErrorWritesEventAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/orders", func(c *gin.Context) {
		Abort(c, errs.NewValidation("order id is required"))
	})

	rec := doRequest(r, "/orders")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应体失败: %v（body=%s）", err, rec.Body.String())
	}
	if resp["code"] != "VALIDATION_ERROR" {
		t.Errorf("code = %v, want VALIDATION_ERROR", resp["code"])
	}
	if resp["message"] != "order id is required" {
		t.Errorf("message = %v, want 校验错误透传", resp["message"])
	}

	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "error.type", "validation.failed")
	attrString(t, attrs, "app.result", "failed")
	attrString(t, attrs, "level", "WARN")
}

// TestMiddlewareSystemErrorHidesMessage 验证系统错误（KindSystem）：
// 状态码 500、响应体固定 SYS_ERROR 消息不泄露内部细节、事件投影为 error 事件（ERROR）。
func TestMiddlewareSystemErrorHidesMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewSystem(errs.TypeDBQueryTimeout, "dial tcp 10.0.0.1:3306: timeout"))
	})

	rec := doRequest(r, "/orders/1")

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
	if resp["message"] != "系统繁忙，请稍后重试" {
		t.Errorf("message = %v, want 固定兜底消息", resp["message"])
	}
	if got := rec.Body.String(); strings.Contains(got, "dial tcp") {
		t.Errorf("响应体不应泄露内部细节: %s", got)
	}

	if len(w.msgs) != 1 || w.msgs[0] != "error" {
		t.Fatalf("msgs = %v, want [error]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "error.http.request")
	attrString(t, attrs, "error.type", "db.query_timeout")
	attrString(t, attrs, "exception.message", "dial tcp 10.0.0.1:3306: timeout")
	attrString(t, attrs, "app.result", "error")
	attrString(t, attrs, "level", "ERROR")
}

// TestMiddlewareNilErrorFallback 验证 Abort(nil)：nil 被替换为固定内部错误
// （error.unknown → 500 SYS_ERROR），避免 gin c.Error(nil) panic。
func TestMiddlewareNilErrorFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, nil)
	})

	rec := doRequest(r, "/orders/1")

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

	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "error.type", "error.unknown")
	attrString(t, attrs, "exception.message", "internal error")
	attrString(t, attrs, "level", "ERROR")
}

// TestMiddlewareNoErrorPassthrough 验证无错误时中间件直接放行：不改响应、不写事件。
func TestMiddlewareNoErrorPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := doRequest(r, "/ok")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want ok", rec.Body.String())
	}
	if len(w.msgs) != 0 {
		t.Errorf("无错误不应写事件，实际 %v", w.msgs)
	}
}

// TestMiddlewareFillsTraceAndSpanFromContext 验证从请求 context 的 span context
// 提取 trace_id / span_id 补充到事件 metadata。
func TestMiddlewareFillsTraceAndSpanFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewBusiness("ORDER.NOT_FOUND", "business.not_found", "order not found"))
	})

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
	req := httptest.NewRequest(http.MethodGet, "/orders/1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "trace_id", "0123456789abcdef0123456789abcdef")
	attrString(t, attrs, "span_id", "0123456789abcdef")
}

// TestMiddlewareWithRecoverNoDoubleWrite 验证与 recovermw 组合：panic 时只有
// recover 写错误事件（error.runtime.panic），errresp 不双写。
func TestMiddlewareWithRecoverNoDoubleWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(recovermw.Middleware(recovermw.Config{Logger: logger}))
	r.Use(Middleware(Config{Logger: logger}))
	r.GET("/boom", func(c *gin.Context) { panic("boom") })

	rec := doRequest(r, "/boom")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(w.msgs) != 1 || w.msgs[0] != "error" {
		t.Fatalf("msgs = %v, want 仅 recover 写 1 条 [error]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "error.runtime.panic")
}

// TestMiddlewareCustomStatusForError 验证 Config.StatusForError 可整体覆盖状态码映射。
func TestMiddlewareCustomStatusForError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{
		Logger:         logger,
		StatusForError: func(error) int { return http.StatusTeapot },
	}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewBusiness("ORDER.CREATE.STOCK_INSUFFICIENT", "business.stock_insufficient", "stock insufficient"))
	})

	rec := doRequest(r, "/orders/1")

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rec.Code)
	}
}

// TestMiddlewareCustomEventName 验证 Config.EventName 可覆盖默认事件名。
func TestMiddlewareCustomEventName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Middleware(Config{
		Logger:    logger,
		EventName: log.NewEventName("error", "http", "custom"),
	}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewValidation("order id is required"))
	})

	rec := doRequest(r, "/orders/1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "error.http.custom")
}
