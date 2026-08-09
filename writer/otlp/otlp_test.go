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
	records       []sdklog.Record
	shutdownCalls int
}

func (p *captureProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }

func (p *captureProcessor) OnEmit(_ context.Context, r *sdklog.Record) error {
	p.records = append(p.records, r.Clone())
	return nil
}

func (p *captureProcessor) Shutdown(context.Context) error {
	p.shutdownCalls++
	return nil
}
func (p *captureProcessor) ForceFlush(context.Context) error { return nil }

// newCaptureWriter 构造使用 captureProcessor 的 Writer（测试专用，绕开真实 exporter）。
func newCaptureWriter(p *captureProcessor) *Writer {
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(p))
	return &Writer{logger: provider.Logger("test"), provider: provider, ownsProvider: true}
}

// TestInjectedProviderOwnership 验证注入的共享 provider 不归 Writer 所有，Close 不会关闭它。
func TestInjectedProviderOwnership(t *testing.T) {
	p := &captureProcessor{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(p))
	w, err := New(context.Background(), WithLoggerProvider(provider))
	if err != nil {
		t.Fatal(err)
	}
	if w.ownsProvider {
		t.Fatal("ownsProvider = true, want false")
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.shutdownCalls != 0 {
		t.Fatalf("Shutdown calls = %d, want 0", p.shutdownCalls)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.shutdownCalls != 1 {
		t.Fatalf("provider Shutdown calls = %d, want 1", p.shutdownCalls)
	}
}

// TestParseEndpoint 验证 host:port 与 http(s) URL 均可用，非法输入会显式报错。
func TestParseEndpoint(t *testing.T) {
	valid := map[string]string{
		"collector:4317":                      "http://collector:4317",
		"127.0.0.1:4317":                      "http://127.0.0.1:4317",
		"[::1]:4317":                          "http://[::1]:4317",
		"http://collector:4317":               "http://collector:4317",
		"https://collector.example.com:4317/": "https://collector.example.com:4317/",
	}
	for endpoint, want := range valid {
		t.Run("valid_"+endpoint, func(t *testing.T) {
			got, err := parseEndpoint(endpoint)
			if err != nil {
				t.Fatalf("parseEndpoint(%q) error = %v", endpoint, err)
			}
			if got != want {
				t.Fatalf("parseEndpoint(%q) = %q, want %q", endpoint, got, want)
			}
		})
	}

	invalid := []string{
		"",
		"collector",
		"collector:",
		":4317",
		"collector:not-a-port",
		"collector:70000",
		"grpc://collector:4317",
		"http://",
		"https://collector:0",
		"https://user@collector:4317",
		"https://collector:4317/v1/logs",
		"https://collector:4317?tenant=mall",
	}
	for _, endpoint := range invalid {
		t.Run("invalid_"+endpoint, func(t *testing.T) {
			if _, err := parseEndpoint(endpoint); err == nil {
				t.Fatalf("parseEndpoint(%q) error = nil, want non-nil", endpoint)
			}
		})
	}
}

// TestNewRejectsInvalidEndpoint 验证非法 endpoint 通过公开构造入口返回错误。
func TestNewRejectsInvalidEndpoint(t *testing.T) {
	provider := sdklog.NewLoggerProvider()
	defer provider.Shutdown(context.Background())
	if _, err := New(context.Background(), WithLoggerProvider(provider), WithEndpoint("collector")); err == nil {
		t.Fatal("New with invalid endpoint error = nil, want non-nil")
	}
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
