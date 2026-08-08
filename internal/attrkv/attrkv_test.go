package attrkv

import (
	"log/slog"
	"testing"
	"time"

	otelog "go.opentelemetry.io/otel/log"
)

func TestSeverityMapping(t *testing.T) {
	cases := []struct {
		level string
		want  string
	}{
		{"DEBUG", "DEBUG"},
		{"INFO", "INFO"},
		{"WARN", "WARN"},
		{"ERROR", "ERROR"},
		{"", "INFO"},
	}
	for _, c := range cases {
		attrs := []slog.Attr{
			slog.String("level", c.level),
			slog.String("event.name", "access.http.request"),
		}
		sev, text := Severity(attrs)
		if text != c.want {
			t.Errorf("level=%q: severity text = %q, want %q", c.level, text, c.want)
		}
		if sev.String() == "" {
			t.Errorf("level=%q: severity 为空", c.level)
		}
	}
}

func TestToKeyValuesTypes(t *testing.T) {
	attrs := []slog.Attr{
		slog.String("http.request.method", "GET"),
		slog.Int("http.response.status_code", 200),
		slog.Bool("mall.retryable", false),
		slog.Float64("latency", 1.5),
		slog.Group("mall.audit", slog.String("actor", "admin")),
	}
	kvs := ToKeyValues(attrs)
	if len(kvs) != 5 {
		t.Fatalf("kvs = %d, want 5", len(kvs))
	}
	if kvs[0].Key != "http.request.method" || kvs[0].Value.AsString() != "GET" {
		t.Errorf("kvs[0] = %v", kvs[0])
	}
	if kvs[1].Value.AsInt64() != 200 {
		t.Errorf("kvs[1] = %v", kvs[1])
	}
	if kvs[2].Value.AsBool() {
		t.Errorf("布尔 false 应保留: %v", kvs[2])
	}
	if kvs[4].Key != "mall.audit" {
		t.Errorf("group 键 = %q, want mall.audit", kvs[4].Key)
	}
}

// TestRecordFieldsExtraction 验证 timestamp/level 映射到顶层字段，保留键从 attrs 剥离。
func TestRecordFieldsExtraction(t *testing.T) {
	ts := time.Date(2026, 8, 8, 21, 34, 56, 0, time.UTC)
	attrs := []slog.Attr{
		slog.Time("timestamp", ts),
		slog.String("level", "WARN"),
		slog.String("event.name", "access.http.request"),
		slog.String("trace_id", "949058d5c20153624d52da3358038026"),
		slog.String("span_id", "bba473e9b3034af6"),
		slog.String("http.request.method", "GET"),
		slog.Int("http.response.status_code", 404),
	}
	rec, rest := Record("access", attrs)
	if got := rec.Timestamp(); !got.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", got, ts)
	}
	if rec.Severity() != otelog.SeverityWarn {
		t.Errorf("Severity = %v, want %v", rec.Severity(), otelog.SeverityWarn)
	}
	if rec.SeverityText() != "WARN" {
		t.Errorf("SeverityText = %q, want WARN", rec.SeverityText())
	}
	if rec.EventName() != "access.http.request" {
		t.Errorf("EventName = %q, want access.http.request", rec.EventName())
	}
	if len(rest) != 2 {
		t.Fatalf("rest = %d attrs, want 2: %v", len(rest), rest)
	}
	for _, a := range rest {
		switch a.Key {
		case "timestamp", "level", "event.name", "trace_id", "span_id":
			t.Errorf("rest 含保留键 %q", a.Key)
		}
	}
}

// TestRecordDefaults 验证缺少 timestamp/level 时的缺省值。
func TestRecordDefaults(t *testing.T) {
	before := time.Now()
	rec, rest := Record("business", []slog.Attr{
		slog.String("event.name", "business.order.paid"),
		slog.String("mall.result", "success"),
	})
	after := time.Now()
	if rec.Timestamp().Before(before) || rec.Timestamp().After(after) {
		t.Errorf("Timestamp 不在写入窗口内: %v", rec.Timestamp())
	}
	if rec.Severity() != otelog.SeverityInfo || rec.SeverityText() != "INFO" {
		t.Errorf("Severity = %v/%q, want INFO", rec.Severity(), rec.SeverityText())
	}
	if rec.EventName() != "business.order.paid" {
		t.Errorf("EventName = %q, want business.order.paid", rec.EventName())
	}
	if len(rest) != 1 || rest[0].Key != "mall.result" {
		t.Fatalf("rest = %v, want 仅 mall.result", rest)
	}
}
