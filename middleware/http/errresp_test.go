package httpmw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

var testErrEventName = log.NewEventName("order", "creation", "rejected")

func TestErrorResponseRendersDefaultBodyAndEmitsEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := ErrorResponse(ErrorConfig{Logger: logger, EventName: testErrEventName})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetError(w, errs.NewBusiness("email_exists", errs.ErrorType("business.auth.email_exists"), "email is already registered"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["code"] != "email_exists" || body["message"] != "email is already registered" {
		t.Fatalf("body = %#v, want flat email_exists", body)
	}
	if len(w.eventTypes) != 1 || w.eventTypes[0] != "business" {
		t.Fatalf("events = %v, want one business event", w.eventTypes)
	}
	attrString(t, attrMap(w.attrsList[0]), "event.name", string(testErrEventName))
}

func TestErrorResponseNoErrorPassesThrough(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := ErrorResponse(ErrorConfig{Logger: logger, EventName: testErrEventName})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if len(w.eventTypes) != 0 {
		t.Fatalf("unexpected events: %v", w.eventTypes)
	}
}

func TestSetErrorFallbackWritesDefaultBody(t *testing.T) {
	rec := httptest.NewRecorder()
	SetError(rec, errs.NewValidation("invalid request"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"VALIDATION_ERROR"`) {
		t.Fatalf("body = %s, want flat VALIDATION_ERROR", rec.Body.String())
	}
}

func TestRecoverCatchesPanicAndEmitsEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := Recover(ErrorConfig{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"SYS_ERROR"`) {
		t.Fatalf("body = %s, want flat SYS_ERROR", rec.Body.String())
	}
	if len(w.eventTypes) != 1 || w.eventTypes[0] != "error" {
		t.Fatalf("events = %v, want one error event", w.eventTypes)
	}
	attrString(t, attrMap(w.attrsList[0]), "event.name", "runtime.panic.occurred")
}

func TestResponseProjectorOverride(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := ErrorResponse(ErrorConfig{
		Logger:    logger,
		EventName: testErrEventName,
		ResponseProjector: func(err error, _ string) (int, any) {
			return http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "invalid_credentials", "message": err.Error()}}
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetError(w, errs.NewBusiness("invalid_credentials", errs.ErrorType("business.auth.invalid_credentials"), "bad"))
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401（由投影决定）", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"invalid_credentials"`) {
		t.Fatalf("body = %s, want nested invalid_credentials", rec.Body.String())
	}
	if len(w.eventTypes) != 1 {
		t.Fatalf("events = %v, want one", w.eventTypes)
	}
}

func TestErrorResponseEventNameResolverTakesPrecedence(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := ErrorResponse(ErrorConfig{
		Logger:    logger,
		EventName: log.NewEventName("checkout", "http", "fixed"),
		EventNameResolver: func(error) log.EventName {
			return log.NewEventName("order", "creation", "rejected")
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		SetError(w, errs.NewValidation("order id is required"))
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/orders", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	attrString(t, attrMap(w.attrsList[0]), "event.name", "order.creation.rejected")
}

func TestNilLoggerRendersOnly(t *testing.T) {
	handler := ErrorResponse(ErrorConfig{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetError(w, errs.NewValidation("invalid request"))
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400（nil logger 仍应渲染）", rec.Code)
	}
}

// TestErrorResponseInputGuardEmitsSecurityAuditEvents 验证 net/http 版 InputGuard：
// 系统错误写出 ErrorEvent 后，guard 返回的 Security/Audit 事件按序补发。
func TestErrorResponseInputGuardEmitsSecurityAuditEvents(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	var gotSummary httperr.InputSummary
	handler := ErrorResponse(ErrorConfig{
		Logger:    logger,
		EventName: testErrEventName,
		InputGuard: func(_ context.Context, _ *http.Request, _ error, s httperr.InputSummary) []log.EventPayload {
			gotSummary = s
			return []log.EventPayload{
				log.SecurityEvent{
					EventMetadata: log.EventMetadata{Level: log.LevelWarn},
					Data:          log.SecurityPayload{EventName: log.EventNameSecurityInputAnomaly, Result: log.ResultBlocked},
				},
				log.AuditEvent{
					EventMetadata: log.EventMetadata{Level: log.LevelInfo},
					Data:          log.AuditPayload{EventName: log.EventNameAuditInputAnomaly, Result: log.ResultBlocked},
				},
			}
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetError(w, errs.NewSystem(errs.TypeDeadlineExceeded, "dial tcp: timeout"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(httperr.WithInputSummary(req.Context(), httperr.InputSummary{Fields: []string{"order_id"}, Hash: "sha256:abc"}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

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
		attrString(t, attrMap(w.attrsList[i]), "event.name", want)
	}
}

// TestRecoverInputGuardEmitsSecurityEvent 验证 net/http Recover 同样支持 InputGuard。
func TestRecoverInputGuardEmitsSecurityEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := Recover(ErrorConfig{
		Logger:    logger,
		EventName: testErrEventName,
		InputGuard: func(_ context.Context, _ *http.Request, _ error, _ httperr.InputSummary) []log.EventPayload {
			return []log.EventPayload{
				log.SecurityEvent{
					EventMetadata: log.EventMetadata{Level: log.LevelWarn},
					Data:          log.SecurityPayload{EventName: log.EventNameSecurityInputAnomaly, Result: log.ResultBlocked},
				},
			}
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(w.eventTypes) != 2 || !reflect.DeepEqual(w.eventTypes, []string{"error", "security"}) {
		t.Fatalf("eventTypes = %v, want [error security]", w.eventTypes)
	}
	attrString(t, attrMap(w.attrsList[1]), "event.name", "input.threat.detected")
}
