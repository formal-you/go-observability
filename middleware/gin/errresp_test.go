package ginmw

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

var testErrEventName = log.NewEventName("order", "creation", "rejected")

// TestErrorResponseBusinessErrorWritesEventAndResponse 验证业务拒绝（KindBusiness）：
// 状态码 409、响应体透传业务码与消息、事件投影为 business 事件（WARN / failed）。
func TestErrorResponseBusinessErrorWritesEventAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{
		Logger:       logger,
		EventName:    testErrEventName,
		GetRequestID: func(*gin.Context) string { return "req-biz-1" },
	}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewBusiness(
			"ORDER.CREATE.STOCK_INSUFFICIENT",
			"FAILED_PRECONDITION",
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

	if len(w.eventTypes) != 1 || w.eventTypes[0] != "business" {
		t.Fatalf("eventTypes = %v, want [business]", w.eventTypes)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", string(testErrEventName))
	attrString(t, attrs, "error.type", "FAILED_PRECONDITION")
	attrString(t, attrs, "error.code", "ORDER.CREATE.STOCK_INSUFFICIENT")
	attrString(t, attrs, "app.business_message", "stock insufficient")
	attrString(t, attrs, "app.result", "failed")
	attrString(t, attrs, "level", "WARN")
	attrString(t, attrs, "request_id", "req-biz-1")
}

// TestErrorResponseValidationErrorWritesEventAndResponse 验证参数校验（KindValidation）：
// 状态码 400、响应体 VALIDATION_ERROR、事件投影为 business 事件。
func TestErrorResponseValidationErrorWritesEventAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{Logger: logger, EventName: testErrEventName}))
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
	attrString(t, attrs, "error.type", "INVALID_ARGUMENT")
	attrString(t, attrs, "app.result", "failed")
	attrString(t, attrs, "level", "WARN")
}

// TestErrorResponseSystemErrorHidesMessage 验证系统错误（KindSystem）：
// 状态码 500、响应体固定 SYS_ERROR 消息不泄露内部细节、事件投影为 error 事件（ERROR）。
func TestErrorResponseSystemErrorHidesMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{Logger: logger, EventName: testErrEventName}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewSystem(errs.TypeDeadlineExceeded, "dial tcp 10.0.0.1:3306: timeout"))
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

	if len(w.eventTypes) != 1 || w.eventTypes[0] != "error" {
		t.Fatalf("eventTypes = %v, want [error]", w.eventTypes)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", string(testErrEventName))
	attrString(t, attrs, "error.type", "DEADLINE_EXCEEDED")
	attrString(t, attrs, "exception.message", "dial tcp 10.0.0.1:3306: timeout")
	attrString(t, attrs, "app.result", "error")
	attrString(t, attrs, "level", "ERROR")
}

// TestAbortNilFallback 验证 Abort(nil)：nil 被替换为固定内部错误
// （error.unknown → 500 SYS_ERROR），避免 gin c.Error(nil) panic。
func TestAbortNilFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{Logger: logger, EventName: testErrEventName}))
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
	attrString(t, attrs, "error.type", "UNKNOWN")
	attrString(t, attrs, "exception.message", "internal error")
	attrString(t, attrs, "level", "ERROR")
}

func TestFallbackInternalErrorIsInitialized(t *testing.T) {
	if fallbackInternalError == nil {
		t.Fatal("fallbackInternalError = nil")
	}
	var appErr errs.AppError
	if !errors.As(fallbackInternalError, &appErr) {
		t.Fatalf("fallbackInternalError type = %T, want errs.AppError", fallbackInternalError)
	}
	if appErr.Kind() != errs.KindSystem || appErr.ErrorType() != errs.TypeUnknown {
		t.Fatalf("fallbackInternalError = kind %q type %q, want system/error.unknown", appErr.Kind(), appErr.ErrorType())
	}
}

// TestErrorResponseNoErrorPassthrough 验证无错误时中间件直接放行：不改响应、不写事件。
func TestErrorResponseNoErrorPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{Logger: logger, EventName: testErrEventName}))
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := doRequest(r, "/ok")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want ok", rec.Body.String())
	}
	if len(w.eventTypes) != 0 {
		t.Errorf("无错误不应写事件，实际 %v", w.eventTypes)
	}
}

// TestErrorResponseFillsTraceAndSpanFromContext 验证从请求 context 的 span context
// 提取 trace_id / span_id 补充到事件 metadata。
func TestErrorResponseFillsTraceAndSpanFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{Logger: logger, EventName: testErrEventName}))
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

// TestErrorResponseWithRecoverNoDoubleWrite 验证与 Recover 组合：panic 时只有
// recover 写错误事件（runtime.panic.occurred），errresp 不双写。
func TestErrorResponseWithRecoverNoDoubleWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(Recover(RecoverConfig{Logger: logger}))
	r.Use(ErrorResponse(ErrorConfig{Logger: logger, EventName: testErrEventName}))
	r.GET("/boom", func(c *gin.Context) { panic("boom") })

	rec := doRequest(r, "/boom")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(w.eventTypes) != 1 || w.eventTypes[0] != "error" {
		t.Fatalf("eventTypes = %v, want 仅 recover 写 1 条 [error]", w.eventTypes)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "runtime.panic.occurred")
}

// TestErrorResponseCustomStatusForError 验证 StatusForError 可整体覆盖状态码映射。
func TestErrorResponseCustomStatusForError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{
		Logger:         logger,
		EventName:      testErrEventName,
		StatusForError: func(error) int { return http.StatusTeapot },
	}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewBusiness("ORDER.CREATE.STOCK_INSUFFICIENT", "FAILED_PRECONDITION", "stock insufficient"))
	})

	rec := doRequest(r, "/orders/1")

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rec.Code)
	}
}

// TestErrorResponseCustomEventName 验证 EventName 可覆盖默认事件名。
func TestErrorResponseCustomEventName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{
		Logger:    logger,
		EventName: log.NewEventName("checkout", "http", "custom"),
	}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewValidation("order id is required"))
	})

	rec := doRequest(r, "/orders/1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "checkout.http.custom")
}

func TestErrorResponseEventNameResolverTakesPrecedence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	r := gin.New()
	r.Use(ErrorResponse(ErrorConfig{
		Logger:    logger,
		EventName: log.NewEventName("checkout", "http", "fixed"),
		EventNameResolver: func(error) log.EventName {
			return log.NewEventName("order", "creation", "rejected")
		},
	}))
	r.GET("/orders/:id", func(c *gin.Context) {
		Abort(c, errs.NewValidation("order id is required"))
	})

	rec := doRequest(r, "/orders/1")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	attrString(t, attrMap(w.attrsList[0]), "event.name", "order.creation.rejected")
}

// TestErrorResponseProjectorOverride 验证 ResponseProjector 可注入自定义契约形状：
// 状态码与响应体由投影决定，错误事件仍照常写出。
func TestErrorResponseProjectorOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(ErrorResponse(ErrorConfig{
		Logger:    logger,
		EventName: testErrEventName,
		ResponseProjector: func(err error, _ string) (int, any) {
			return http.StatusUnauthorized, gin.H{"error": gin.H{"code": "invalid_credentials", "message": err.Error()}}
		},
	}))
	engine.GET("/login", func(c *gin.Context) {
		Abort(c, errs.NewBusiness("invalid_credentials", errs.ErrorType("business.auth.invalid_credentials"), "email or password is incorrect"))
	})
	rec := doRequest(engine, "/login")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401（由投影决定）", rec.Code)
	}
	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"]["code"] != "invalid_credentials" || body["error"]["message"] == "" {
		t.Fatalf("body = %#v, want nested invalid_credentials", body)
	}
	if len(w.eventTypes) != 1 || w.eventTypes[0] != "business" {
		t.Fatalf("events = %v, want one business event", w.eventTypes)
	}
}

// TestErrorResponseInputGuardEmitsSecurityAuditEvents 验证 InputGuard 注入点：
// 系统错误写出 ErrorEvent 后，guard 返回的 Security/Audit 事件按序补发，
// 且共享同一 trace/span（扁平投影 trace_id/span_id 一致）。
func TestErrorResponseInputGuardEmitsSecurityAuditEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	var gotSummary httperr.InputSummary
	engine := gin.New()
	engine.Use(ErrorResponse(ErrorConfig{
		Logger:    logger,
		EventName: testErrEventName,
		InputGuard: func(ctx context.Context, _ *http.Request, _ error, s httperr.InputSummary) []log.EventPayload {
			gotSummary = s
			md := httperr.EventMetadataFromContext(ctx)
			return []log.EventPayload{
				log.SecurityEvent{
					EventMetadata: md,
					Data:          log.SecurityPayload{EventName: log.EventNameSecurityInputAnomaly, Result: log.ResultBlocked},
				},
				log.AuditEvent{
					EventMetadata: md,
					Data:          log.AuditPayload{EventName: log.EventNameAuditInputAnomaly, Result: log.ResultBlocked},
				},
			}
		},
	}))
	engine.GET("/x", func(c *gin.Context) {
		c.Request = c.Request.WithContext(httperr.WithInputSummary(c.Request.Context(), httperr.InputSummary{Fields: []string{"order_id"}, Hash: "sha256:abc"}))
		Abort(c, errs.NewSystem(errs.TypeDeadlineExceeded, "dial tcp: timeout"))
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
	req := httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(trace.ContextWithRemoteSpanContext(context.Background(), sc))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !reflect.DeepEqual(gotSummary.Fields, []string{"order_id"}) || gotSummary.Hash != "sha256:abc" {
		t.Errorf("guard 摘要 = %+v, want order_id/sha256:abc", gotSummary)
	}
	if len(w.eventTypes) != 3 || !reflect.DeepEqual(w.eventTypes, []string{"error", "security", "audit"}) {
		t.Fatalf("eventTypes = %v, want [error security audit]", w.eventTypes)
	}
	for i, want := range []string{string(testErrEventName), "input.threat.detected", "input.anomaly.recorded"} {
		attrs := attrMap(w.attrsList[i])
		attrString(t, attrs, "event.name", want)
		attrString(t, attrs, "trace_id", "0123456789abcdef0123456789abcdef")
		attrString(t, attrs, "span_id", "0123456789abcdef")
	}
}

// TestRecoverInputGuardEmitsSecurityEvent 验证 Recover 同样支持 InputGuard：
// panic 收口写出 ErrorEvent 后按序补发 SecurityEvent。
func TestRecoverInputGuardEmitsSecurityEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(Recover(RecoverConfig{
		Logger: logger,
		InputGuard: func(_ context.Context, _ *http.Request, _ error, _ httperr.InputSummary) []log.EventPayload {
			return []log.EventPayload{
				log.SecurityEvent{
					EventMetadata: log.EventMetadata{Level: log.LevelWarn},
					Data:          log.SecurityPayload{EventName: log.EventNameSecurityInputAnomaly, Result: log.ResultBlocked},
				},
			}
		},
	}))
	engine.GET("/boom", func(c *gin.Context) { panic("boom") })

	rec := doRequest(engine, "/boom")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(w.eventTypes) != 2 || !reflect.DeepEqual(w.eventTypes, []string{"error", "security"}) {
		t.Fatalf("eventTypes = %v, want [error security]", w.eventTypes)
	}
	attrString(t, attrMap(w.attrsList[1]), "event.name", "input.threat.detected")
}
