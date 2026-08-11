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

func TestNewLoggerRejectsNilWriter(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewLogger(nil) 应 panic")
		}
	}()
	NewLogger(nil)
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
	for _, k := range []string{"timestamp", "trace_id", "span_id", "event.name", "http.request.method", "app.result"} {
		if _, ok := attrs[k]; !ok {
			t.Errorf("缺少属性 %s（实际: %v）", k, keysOf(attrs))
		}
	}
}

func TestLoggerBaseMetadataFillsMissing(t *testing.T) {
	w := &captureWriter{}
	l := NewLogger(w, WithBaseMetadata(EventMetadata{Level: LevelWarn, RequestID: "base-req"}))
	l.Emit(context.Background(), BusinessEvent{
		Data: BusinessPayload{EventName: EventName("order.payment.succeeded"), Result: ResultSuccess},
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
		Data: BusinessPayload{EventName: EventName("order.payment.succeeded"), Result: ResultSuccess},
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
		Data: BusinessPayload{EventName: EventName("order.payment.succeeded"), Result: ResultSuccess},
	})
	if got == nil {
		t.Error("writer 失败时 error handler 应被调用")
	}
}

func TestLoggerTraceExtractorFillsMissing(t *testing.T) {
	w := &captureWriter{}
	l := NewLogger(w, WithTraceExtractor(TraceExtractorFunc(func(_ context.Context) TraceContext {
		return TraceContext{TraceID: "abcdef0123456789abcdef0123456789", SpanID: "0123456789abcdef"}
	})))
	l.Emit(context.Background(), BusinessEvent{
		Data: BusinessPayload{EventName: EventName("order.payment.succeeded"), Result: ResultSuccess},
	})
	attrs := attrMap(w.attrsList[0])
	if got := attrs["trace_id"].(slog.Value).String(); got != "abcdef0123456789abcdef0123456789" {
		t.Errorf("trace_id 应由 extractor 补全，实际: %v", got)
	}
	if got := attrs["span_id"].(slog.Value).String(); got != "0123456789abcdef" {
		t.Errorf("span_id 应由 extractor 补全，实际: %v", got)
	}
}

func TestLoggerTraceExtractorDoesNotOverride(t *testing.T) {
	w := &captureWriter{}
	l := NewLogger(w, WithTraceExtractor(TraceExtractorFunc(func(_ context.Context) TraceContext {
		return TraceContext{TraceID: "from-extractor", SpanID: "from-extractor"}
	})))
	l.Emit(context.Background(), BusinessEvent{
		EventMetadata: EventMetadata{TraceID: "from-event", SpanID: "from-event"},
		Data:          BusinessPayload{EventName: EventName("order.payment.succeeded"), Result: ResultSuccess},
	})
	attrs := attrMap(w.attrsList[0])
	if got := attrs["trace_id"].(slog.Value).String(); got != "from-event" {
		t.Errorf("事件显式设置的 trace_id 不应被覆盖，实际: %v", got)
	}
	if got := attrs["span_id"].(slog.Value).String(); got != "from-event" {
		t.Errorf("事件显式设置的 span_id 不应被覆盖，实际: %v", got)
	}
}

func TestLoggerMinLevel(t *testing.T) {
	w := &captureWriter{}
	l := NewLogger(w, WithMinLevel(LevelWarn))
	l.Emit(context.Background(), BusinessEvent{
		EventMetadata: EventMetadata{Level: LevelInfo},
		Data:          BusinessPayload{EventName: EventName("order.payment.succeeded"), Result: ResultSuccess},
	})
	l.Emit(context.Background(), ErrorEvent{
		EventMetadata: EventMetadata{Level: LevelError},
		Data: ErrorPayload{
			EventName: EventNameErrorDBTimeout,
			ErrorType: "db.timeout",
			Result:    ResultError,
		},
	})
	if len(w.msgs) != 1 || w.msgs[0] != "error" {
		t.Fatalf("WARN 最低级别应丢弃 INFO、保留 ERROR，实际 %v", w.msgs)
	}
}

func TestWithMinLevelRejectsInvalidLevel(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("非法最低级别应 panic")
		}
	}()
	_ = WithMinLevel(Level("NOTICE"))
}
