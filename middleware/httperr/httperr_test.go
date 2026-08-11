package httperr

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/errs"
)

func TestStatusForError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"validation", errs.NewValidation("bad param"), http.StatusBadRequest},
		{"business", errs.NewBusiness("ORDER.CREATE.STOCK_INSUFFICIENT", errs.ErrorType("business.order.stock_insufficient"), "库存不足"), http.StatusConflict},
		{"system", errs.NewSystem(errs.TypeDBConnectionError, "connection refused"), http.StatusInternalServerError},
		{"plain", errors.New("boom"), http.StatusInternalServerError},
		{"nil", nil, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StatusForError(tc.err); got != tc.want {
				t.Errorf("StatusForError(%q) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		reason, msg, md := ClassifyError(errs.NewValidation("用户名为空"))
		if reason != "validation_error" || msg != "用户名为空" {
			t.Errorf("got (%q,%q), want validation_error/用户名为空", reason, msg)
		}
		if md["error.type"] != "validation.failed" {
			t.Errorf("metadata = %v, want error.type=validation.failed", md)
		}
	})
	t.Run("business with code", func(t *testing.T) {
		reason, msg, _ := ClassifyError(errs.NewBusiness("ORDER.CREATE.STOCK_INSUFFICIENT", errs.ErrorType("business.order.stock_insufficient"), "库存不足"))
		if reason != "ORDER.CREATE.STOCK_INSUFFICIENT" || msg != "库存不足" {
			t.Errorf("got (%q,%q), want business code/message", reason, msg)
		}
	})
	t.Run("business without code", func(t *testing.T) {
		reason, _, _ := ClassifyError(errs.NewBusiness("", errs.ErrorType("business.x"), "x"))
		if reason != "business_error" {
			t.Errorf("reason = %q, want business_error", reason)
		}
	})
	t.Run("system does not leak detail", func(t *testing.T) {
		reason, msg, md := ClassifyError(errs.NewSystem(errs.TypeDBConnectionError, "dial tcp 127.0.0.1:5432: connection refused"))
		if reason != "system_error" || msg != "internal server error" {
			t.Errorf("got (%q,%q), want system_error/internal server error", reason, msg)
		}
		if md["error.type"] != "db.connection_error" {
			t.Errorf("metadata = %v, want error.type=db.connection_error", md)
		}
	})
	t.Run("plain error fallback", func(t *testing.T) {
		reason, msg, md := ClassifyError(errors.New("boom"))
		if reason != "system_error" || msg != "internal server error" {
			t.Errorf("got (%q,%q), want system_error/internal server error", reason, msg)
		}
		if md["error.type"] != "error.unknown" {
			t.Errorf("metadata = %v, want error.type=error.unknown", md)
		}
	})
}

func TestResponseBody(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		body := ResponseBody(errs.NewValidation("bad"), "")
		if body["code"] != "VALIDATION_ERROR" || body["message"] != "bad" {
			t.Errorf("body = %v", body)
		}
	})
	t.Run("business", func(t *testing.T) {
		body := ResponseBody(errs.NewBusiness("ORDER.CREATE.STOCK_INSUFFICIENT", errs.ErrorType("business.x"), "库存不足"), "")
		if body["code"] != "ORDER.CREATE.STOCK_INSUFFICIENT" || body["message"] != "库存不足" {
			t.Errorf("body = %v", body)
		}
	})
	t.Run("business empty code", func(t *testing.T) {
		body := ResponseBody(errs.NewBusiness("", errs.ErrorType("business.x"), "x"), "")
		if body["code"] != "BIZ_ERROR" {
			t.Errorf("code = %v, want BIZ_ERROR", body["code"])
		}
	})
	t.Run("system fixed", func(t *testing.T) {
		body := ResponseBody(errs.NewSystem(errs.TypeDBConnectionError, "secret detail"), "")
		if body["code"] != "SYS_ERROR" || body["message"] != "系统繁忙，请稍后重试" {
			t.Errorf("body = %v", body)
		}
	})
	t.Run("plain", func(t *testing.T) {
		body := ResponseBody(errors.New("boom"), "")
		if body["code"] != "SYS_ERROR" {
			t.Errorf("code = %v, want SYS_ERROR", body["code"])
		}
	})
	t.Run("request id", func(t *testing.T) {
		body := ResponseBody(errors.New("boom"), "req-1")
		if body["request_id"] != "req-1" {
			t.Errorf("request_id = %v", body["request_id"])
		}
	})
}

func TestDefaultProjector(t *testing.T) {
	err := errs.NewBusiness("ORDER.CREATE.STOCK_INSUFFICIENT", errs.ErrorType("business.x"), "库存不足")
	status, body := DefaultProjector(err, "req-1")
	if status != http.StatusConflict {
		t.Errorf("status = %d, want 409", status)
	}
	b, ok := body.(map[string]any)
	if !ok || b["request_id"] != "req-1" {
		t.Errorf("body = %v, want request_id present", body)
	}
}

func TestEventMetadataFromContext(t *testing.T) {
	t.Run("no span", func(t *testing.T) {
		md := EventMetadataFromContext(context.Background())
		if md.TraceID != "" || md.SpanID != "" {
			t.Errorf("md = %+v, want empty", md)
		}
	})
	t.Run("valid span", func(t *testing.T) {
		tid, err := trace.TraceIDFromHex("949058d5c20153624d52da3358038026")
		if err != nil {
			t.Fatal(err)
		}
		sid, err := trace.SpanIDFromHex("bba473e9b3034af6")
		if err != nil {
			t.Fatal(err)
		}
		sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled})
		ctx := trace.ContextWithRemoteSpanContext(context.Background(), sc)
		md := EventMetadataFromContext(ctx)
		if md.TraceID != tid.String() || md.SpanID != sid.String() {
			t.Errorf("md = %+v, want trace/span %s/%s", md, tid.String(), sid.String())
		}
	})
}

func TestSystemErrorFromPanic(t *testing.T) {
	err := SystemErrorFromPanic(errs.TypeRuntimePanic, "boom: nil pointer")
	if err.Kind() != errs.KindSystem {
		t.Errorf("Kind = %q, want system", err.Kind())
	}
	if err.ErrorType() != errs.TypeRuntimePanic {
		t.Errorf("ErrorType = %q, want runtime.panic", err.ErrorType())
	}
	if err.Error() != "boom: nil pointer" {
		t.Errorf("Error = %q, want panic value", err.Error())
	}
	if !strings.Contains(err.Stack(), "goroutine") {
		t.Errorf("stack should contain goroutine dump")
	}
}
