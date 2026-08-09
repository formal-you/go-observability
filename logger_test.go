package log

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

type captureWriter struct {
	msgs      []string
	attrsList [][]slog.Attr
	err       error
}

func (w *captureWriter) Write(_ context.Context, msg string, attrs ...slog.Attr) error {
	w.msgs = append(w.msgs, msg)
	w.attrsList = append(w.attrsList, attrs)
	return w.err
}

func TestLoggerEmitWritesNormalizedAttrs(t *testing.T) {
	w := &captureWriter{}
	l := NewLogger(w)
	ev := AccessEvent{
		EventMetadata: EventMetadata{
			Level:     LevelInfo,
			TraceID:   "0123456789abcdef0123456789abcdef",
			SpanID:    "0123456789abcdef",
			RequestID: "req-001",
		},
		Data: AccessPayload{
			EventName: EventNameAccessHTTPRequest,
			HTTP:      HTTPInfo{Method: "GET", StatusCode: 200},
			Result:    ResultSuccess,
		},
	}
	l.Emit(context.Background(), ev)
	if len(w.msgs) != 1 || w.msgs[0] != "access" {
		t.Fatalf("msg = %v, want [access]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	for _, k := range []string{"trace_id", "span_id", "event.name", "http.request.method", "app.result"} {
		if _, ok := attrs[k]; !ok {
			t.Errorf("缺少属性 %s（实际: %v）", k, keysOf(attrs))
		}
	}
}

func TestLoggerBaseMetadataFillsMissing(t *testing.T) {
	w := &captureWriter{}
	l := NewLogger(w, WithBaseMetadata(EventMetadata{Level: LevelWarn, RequestID: "base-req"}))
	l.Emit(context.Background(), BusinessEvent{
		Data: BusinessPayload{EventName: EventNameBusinessOrderPaid, Result: ResultSuccess},
	})
	attrs := attrMap(w.attrsList[0])
	if attrs["level"] == nil {
		t.Error("base metadata 应补全缺失的 level")
	}
	if attrs["request_id"] == nil {
		t.Error("base metadata 应补全缺失的 request_id")
	}
}

func TestLoggerMaskerAndSampler(t *testing.T) {
	w := &captureWriter{}
	l := NewLogger(w,
		WithMasker(MaskerFunc(func(_ context.Context, attrs []slog.Attr) []slog.Attr {
			return append(attrs[:0:0], slog.String("masked", "1"))
		})),
		WithSampler(SamplerFunc(func(_ context.Context, _ []slog.Attr) bool { return false })),
	)
	l.Emit(context.Background(), BusinessEvent{
		Data: BusinessPayload{EventName: EventNameBusinessOrderPaid, Result: ResultSuccess},
	})
	if len(w.msgs) != 0 {
		t.Error("sampler 返回 false 时不应写入")
	}
}

func TestLoggerErrorHandler(t *testing.T) {
	w := &captureWriter{err: errors.New("write failed")}
	var got error
	l := NewLogger(w, WithErrorHandler(func(_ context.Context, _ string, _ []slog.Attr, err error) {
		got = err
	}))
	l.Emit(context.Background(), BusinessEvent{
		Data: BusinessPayload{EventName: EventNameBusinessOrderPaid, Result: ResultSuccess},
	})
	if got == nil {
		t.Error("writer 失败时 error handler 应被调用")
	}
}
