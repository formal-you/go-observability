package httpmw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
)

func TestErrorResponseRendersDefaultBodyAndEmitsEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := ErrorResponse(ErrorConfig{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if len(w.msgs) != 1 || w.msgs[0] != "business" {
		t.Fatalf("events = %v, want one business event", w.msgs)
	}
	attrString(t, attrMap(w.attrsList[0]), "event.name", "error.http.request")
}

func TestErrorResponseNoErrorPassesThrough(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := ErrorResponse(ErrorConfig{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if len(w.msgs) != 0 {
		t.Fatalf("unexpected events: %v", w.msgs)
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
	if len(w.msgs) != 1 || w.msgs[0] != "error" {
		t.Fatalf("events = %v, want one error event", w.msgs)
	}
	attrString(t, attrMap(w.attrsList[0]), "event.name", "error.runtime.panic")
}

func TestResponseProjectorOverride(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := ErrorResponse(ErrorConfig{
		Logger: logger,
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
	if len(w.msgs) != 1 {
		t.Fatalf("events = %v, want one", w.msgs)
	}
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
