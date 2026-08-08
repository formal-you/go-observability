package attrkv

import (
	"log/slog"
	"testing"
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
