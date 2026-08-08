package otlp

import (
	"context"
	"log/slog"
	"testing"
	"time"

	otelog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

// captureProcessor 捕获 OnEmit 收到的 Record，用于断言 LogRecord 形状，不依赖真实 Collector。
type captureProcessor struct {
	records []sdklog.Record
}

func (p *captureProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }

func (p *captureProcessor) OnEmit(_ context.Context, r *sdklog.Record) error {
	p.records = append(p.records, r.Clone())
	return nil
}

func (p *captureProcessor) Shutdown(context.Context) error   { return nil }
func (p *captureProcessor) ForceFlush(context.Context) error { return nil }

// newCaptureWriter 构造使用 captureProcessor 的 Writer（测试专用，绕开真实 exporter）。
func newCaptureWriter(p *captureProcessor) *Writer {
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(p))
	return &Writer{logger: provider.Logger("test"), provider: provider}
}

// TestWriteRecordShape 验证 timestamp/level 映射到顶层字段，保留键不写入属性。
func TestWriteRecordShape(t *testing.T) {
	p := &captureProcessor{}
	w := newCaptureWriter(p)
	defer w.Close(context.Background())

	ts := time.Date(2026, 8, 8, 21, 34, 56, 0, time.UTC)
	if err := w.Write(context.Background(), "access",
		slog.Time("timestamp", ts),
		slog.String("level", "WARN"),
		slog.String("event.name", "access.http.request"),
		slog.String("trace_id", "949058d5c20153624d52da3358038026"),
		slog.String("span_id", "bba473e9b3034af6"),
		slog.String("http.request.method", "GET"),
	); err != nil {
		t.Fatal(err)
	}
	if len(p.records) != 1 {
		t.Fatalf("records = %d, want 1", len(p.records))
	}
	rec := p.records[0]
	if !rec.Timestamp().Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", rec.Timestamp(), ts)
	}
	if rec.Severity() != otelog.SeverityWarn || rec.SeverityText() != "WARN" {
		t.Errorf("Severity = %v / %q, want WARN", rec.Severity(), rec.SeverityText())
	}
	if rec.EventName() != "access.http.request" {
		t.Errorf("EventName = %q, want access.http.request", rec.EventName())
	}

	keys := recordKeys(&rec)
	for _, reserved := range []string{"timestamp", "level", "event.name", "trace_id", "span_id"} {
		if hasKey(keys, reserved) {
			t.Errorf("属性含保留键 %q: %v", reserved, keys)
		}
	}
	if !hasKey(keys, "http.request.method") {
		t.Errorf("属性缺 http.request.method: %v", keys)
	}
}

// TestWriteTraceContextFromSpan 验证 ctx 中的 span context 自动关联到 LogRecord。
func TestWriteTraceContextFromSpan(t *testing.T) {
	p := &captureProcessor{}
	w := newCaptureWriter(p)
	defer w.Close(context.Background())

	tid := trace.TraceID{0x01}
	sid := trace.SpanID{0x02}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	if err := w.Write(ctx, "access", slog.String("http.request.method", "GET")); err != nil {
		t.Fatal(err)
	}
	if len(p.records) != 1 {
		t.Fatalf("records = %d, want 1", len(p.records))
	}
	rec := p.records[0]
	if rec.TraceID() != tid || rec.SpanID() != sid {
		t.Errorf("TraceID/SpanID = %s/%s, want %s/%s", rec.TraceID(), rec.SpanID(), tid, sid)
	}
}

// TestWriteSmoke 仅验证 New/Write/Close 不报错：导出为异步且失败不回传（采集由 Collector 控制）。
func TestWriteSmoke(t *testing.T) {
	w, err := New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())
	if err := w.Write(context.Background(), "access", slog.String("http.request.method", "GET")); err != nil {
		t.Fatal(err)
	}
}

func recordKeys(rec *sdklog.Record) []string {
	var keys []string
	rec.WalkAttributes(func(kv otelog.KeyValue) bool {
		keys = append(keys, kv.Key)
		return true
	})
	return keys
}

func hasKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}
