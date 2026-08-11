package kratosmw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
)

type captureWriter struct {
	attrsList [][]slog.Attr
}

func (w *captureWriter) Write(_ context.Context, _ string, attrs ...slog.Attr) error {
	w.attrsList = append(w.attrsList, attrs)
	return nil
}

func attrMapOf(attrs []slog.Attr) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value.Any()
	}
	return m
}

func encode(t *testing.T, enc func(w http.ResponseWriter, r *http.Request, err error), err error) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	enc(rec, httptest.NewRequest(http.MethodGet, "/", nil), err)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
	}
	return rec.Code, body
}

func TestErrorEncoder(t *testing.T) {
	t.Run("business AppError", func(t *testing.T) {
		err := errs.NewBusiness("todo_not_found", errs.ErrorType("business.todo.not_found"), "todo not found")
		status, body := encode(t, ErrorEncoder(), err)
		if status != http.StatusConflict {
			t.Fatalf("status = %d, want 409", status)
		}
		if body["reason"] != "todo_not_found" || body["message"] != "todo not found" {
			t.Fatalf("body = %v", body)
		}
		md, _ := body["metadata"].(map[string]any)
		if md["error.type"] != "business.todo.not_found" {
			t.Fatalf("metadata = %v", body["metadata"])
		}
	})
	t.Run("validation AppError", func(t *testing.T) {
		err := errs.NewValidation("invalid request")
		status, body := encode(t, ErrorEncoder(), err)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
		if body["reason"] != "validation_error" || body["message"] != "invalid request" {
			t.Fatalf("body = %v", body)
		}
	})
	t.Run("system AppError hides detail", func(t *testing.T) {
		err := errs.NewSystem(errs.TypeDBQueryTimeout, "find user: context deadline exceeded")
		status, body := encode(t, ErrorEncoder(), err)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", status)
		}
		if body["message"] != "internal server error" {
			t.Fatalf("message = %v, want fixed（不透传内部细节）", body["message"])
		}
		md, _ := body["metadata"].(map[string]any)
		if md["error.type"] != "db.query_timeout" {
			t.Fatalf("metadata = %v", body["metadata"])
		}
	})
	t.Run("plain error hides detail", func(t *testing.T) {
		status, body := encode(t, ErrorEncoder(), errors.New("boom"))
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", status)
		}
		if body["message"] != "internal server error" {
			t.Fatalf("message = %v, want fixed", body["message"])
		}
		md, _ := body["metadata"].(map[string]any)
		if md["error.type"] != "error.unknown" {
			t.Fatalf("metadata = %v", body["metadata"])
		}
	})
	t.Run("kratos native error passes through", func(t *testing.T) {
		err := kerrors.NotFound("TODO_NOT_FOUND", "todo not found")
		status, body := encode(t, ErrorEncoder(), err)
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", status)
		}
		if body["reason"] != "TODO_NOT_FOUND" || body["message"] != "todo not found" {
			t.Fatalf("body = %v", body)
		}
	})
	t.Run("wrapped AppError still found", func(t *testing.T) {
		inner := errs.NewBusiness("todo_not_found", errs.ErrorType("business.todo.not_found"), "todo not found")
		status, body := encode(t, ErrorEncoder(), fmt.Errorf("get todo: %w", inner))
		if status != http.StatusConflict || body["reason"] != "todo_not_found" {
			t.Fatalf("status=%d body=%v", status, body)
		}
	})
	t.Run("wrapped kratos error still found", func(t *testing.T) {
		inner := kerrors.NotFound("TODO_NOT_FOUND", "todo not found")
		status, body := encode(t, ErrorEncoder(), fmt.Errorf("get todo: %w", inner))
		if status != http.StatusNotFound || body["reason"] != "TODO_NOT_FOUND" {
			t.Fatalf("status=%d body=%v", status, body)
		}
	})
}

func TestErrorLog(t *testing.T) {
	t.Run("emits event and propagates error", func(t *testing.T) {
		w := &captureWriter{}
		logger := log.NewLogger(w)
		mw := ErrorLog(logger)

		wantErr := errs.NewBusiness("todo_not_found", errs.ErrorType("business.todo.not_found"), "todo not found")
		next := func(context.Context, any) (any, error) { return nil, wantErr }
		reply, err := mw(next)(context.Background(), nil)

		if !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want 原样透传", err)
		}
		if reply != nil {
			t.Fatalf("reply = %v, want nil", reply)
		}
		if len(w.attrsList) != 1 {
			t.Fatalf("events = %d, want 1", len(w.attrsList))
		}
		attrs := attrMapOf(w.attrsList[0])
		if attrs["event.name"] != "error.http.request" {
			t.Fatalf("event.name = %v", attrs["event.name"])
		}
		if attrs["error.type"] != "business.todo.not_found" {
			t.Fatalf("error.type = %v", attrs["error.type"])
		}
		if attrs["app.result"] != "failed" {
			t.Fatalf("app.result = %v", attrs["app.result"])
		}
	})
	t.Run("nil logger is passthrough", func(t *testing.T) {
		mw := ErrorLog(nil)
		wantErr := errors.New("boom")
		_, err := mw(func(context.Context, any) (any, error) { return nil, wantErr })(context.Background(), nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want 透传", err)
		}
	})
	t.Run("success emits nothing", func(t *testing.T) {
		w := &captureWriter{}
		logger := log.NewLogger(w)
		mw := ErrorLog(logger)
		_, err := mw(func(context.Context, any) (any, error) { return "ok", nil })(context.Background(), nil)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if len(w.attrsList) != 0 {
			t.Fatalf("events = %d, want 0", len(w.attrsList))
		}
	})
}

func TestGRPCErrorMapper(t *testing.T) {
	t.Run("business AppError maps to status with ErrorInfo", func(t *testing.T) {
		err := errs.NewBusiness("todo_not_found", errs.ErrorType("business.todo.not_found"), "todo not found")
		mw := GRPCErrorMapper()
		_, got := mw(func(context.Context, any) (any, error) { return nil, err })(context.Background(), nil)

		st, ok := status.FromError(got)
		if !ok {
			t.Fatalf("err = %v, 不是 gRPC status error", got)
		}
		if st.Code() != codes.Aborted {
			t.Fatalf("code = %v, want Aborted（409→kratos 映射）", st.Code())
		}
		var found bool
		for _, detail := range st.Details() {
			if ei, ok := detail.(*errdetails.ErrorInfo); ok {
				found = true
				if ei.Reason != "todo_not_found" || ei.Metadata["error.type"] != "business.todo.not_found" {
					t.Fatalf("ErrorInfo = %+v", ei)
				}
			}
		}
		if !found {
			t.Fatal("缺少 errdetails.ErrorInfo")
		}
	})
	t.Run("validation AppError maps to InvalidArgument", func(t *testing.T) {
		err := errs.NewValidation("invalid request")
		mw := GRPCErrorMapper()
		_, got := mw(func(context.Context, any) (any, error) { return nil, err })(context.Background(), nil)
		if st, _ := status.FromError(got); st.Code() != codes.InvalidArgument {
			t.Fatalf("code = %v, want InvalidArgument", st.Code())
		}
	})
	t.Run("system AppError hides detail", func(t *testing.T) {
		err := errs.NewSystem(errs.TypeDBQueryTimeout, "find user: context deadline exceeded")
		mw := GRPCErrorMapper()
		_, got := mw(func(context.Context, any) (any, error) { return nil, err })(context.Background(), nil)
		st, _ := status.FromError(got)
		if st.Code() != codes.Internal || st.Message() != "internal server error" {
			t.Fatalf("code=%v message=%q, want Internal + 固定文案", st.Code(), st.Message())
		}
	})
	t.Run("plain error hides detail", func(t *testing.T) {
		mw := GRPCErrorMapper()
		_, got := mw(func(context.Context, any) (any, error) { return nil, errors.New("boom") })(context.Background(), nil)
		st, _ := status.FromError(got)
		if st.Code() != codes.Internal || st.Message() != "internal server error" {
			t.Fatalf("code=%v message=%q, want Internal + 固定文案", st.Code(), st.Message())
		}
	})
	t.Run("kratos native error passes through", func(t *testing.T) {
		inner := kerrors.NotFound("TODO_NOT_FOUND", "todo not found")
		mw := GRPCErrorMapper()
		_, got := mw(func(context.Context, any) (any, error) { return nil, inner })(context.Background(), nil)
		if got != inner {
			t.Fatalf("err 被改写: %T %v, want 原样", got, got)
		}
	})
	t.Run("success returns nil", func(t *testing.T) {
		mw := GRPCErrorMapper()
		reply, err := mw(func(context.Context, any) (any, error) { return "ok", nil })(context.Background(), nil)
		if err != nil || reply != "ok" {
			t.Fatalf("reply=%v err=%v", reply, err)
		}
	})
}
