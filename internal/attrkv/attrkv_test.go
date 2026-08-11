package attrkv

import (
	"log/slog"
	"testing"
	"time"

	corelog "github.com/formal-you/go-observability/log"
	"go.opentelemetry.io/otel/attribute"
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
		slog.Bool("app.retryable", false),
		slog.Float64("latency", 1.5),
		slog.Group("app.audit", slog.String("actor", "admin")),
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
	if kvs[4].Key != "app.audit" {
		t.Errorf("group 键 = %q, want app.audit", kvs[4].Key)
	}
	if kvs[4].Value.Type() != attribute.MAP {
		t.Fatalf("group kind = %v, want Map", kvs[4].Value.Type())
	}
	group := kvs[4].Value.AsMap()
	if len(group) != 1 || group[0].Key != "actor" || group[0].Value.AsString() != "admin" {
		t.Errorf("group value = %v, want actor=admin", group)
	}
}

type testLogValuer struct{}

func (testLogValuer) LogValue() slog.Value {
	return slog.GroupValue(slog.String("resolved", "yes"))
}

func TestToKeyValuesAnyRecursive(t *testing.T) {
	kvs := ToKeyValues([]slog.Attr{slog.Any("payload", map[string]any{
		"active": true,
		"count":  3,
		"labels": []string{"mall", "checkout"},
		"items":  []any{"first", int64(2), map[string]any{"ok": false}},
		"valuer": testLogValuer{},
	})})
	if len(kvs) != 1 || kvs[0].Value.Type() != attribute.MAP {
		t.Fatalf("payload = %v, want one Map value", kvs)
	}

	payload := mapValues(kvs[0].Value.AsMap())
	if payload["active"].Type() != attribute.BOOL || !payload["active"].AsBool() {
		t.Errorf("active = %v, want Bool(true)", payload["active"])
	}
	if payload["count"].Type() != attribute.INT64 || payload["count"].AsInt64() != 3 {
		t.Errorf("count = %v, want Int64(3)", payload["count"])
	}
	labels := payload["labels"]
	if labels.Type() != attribute.SLICE || len(labels.AsSlice()) != 2 || labels.AsSlice()[1].AsString() != "checkout" {
		t.Errorf("labels = %v, want [mall checkout]", labels)
	}
	items := payload["items"]
	if items.Type() != attribute.SLICE || len(items.AsSlice()) != 3 {
		t.Fatalf("items = %v, want three-item Slice", items)
	}
	if items.AsSlice()[2].Type() != attribute.MAP {
		t.Fatalf("items[2] = %v, want Map", items.AsSlice()[2])
	}
	itemMap := mapValues(items.AsSlice()[2].AsMap())
	if itemMap["ok"].Type() != attribute.BOOL || itemMap["ok"].AsBool() {
		t.Errorf("items[2] = %v, want Map(ok=false)", items.AsSlice()[2])
	}
	valuer := payload["valuer"]
	if valuer.Type() != attribute.MAP || mapValues(valuer.AsMap())["resolved"].AsString() != "yes" {
		t.Errorf("valuer = %v, want resolved Map", valuer)
	}
}

func TestAuditEventFieldsRemainStructured(t *testing.T) {
	ev := corelog.AuditEvent{
		EventMetadata: corelog.EventMetadata{Level: corelog.LevelWarn},
		Data: corelog.AuditPayload{
			EventName: corelog.EventName("audit.user.role_changed"),
			Before: corelog.Fields{
				"role": "viewer",
				"metadata": corelog.Fields{
					"approved": false,
				},
			},
			After:  corelog.Fields{"role": "admin"},
			Result: corelog.ResultSuccess,
		},
	}

	_, attrs := Record("audit", ev.Attrs())
	values := mapValues(ToKeyValues(attrs))
	before := values[string(corelog.KeyAppBefore)]
	after := values[string(corelog.KeyAppAfter)]
	if before.Type() != attribute.MAP || after.Type() != attribute.MAP {
		t.Fatalf("before/after kind = %v/%v, want Map/Map", before.Type(), after.Type())
	}
	beforeValues := mapValues(before.AsMap())
	if beforeValues["role"].AsString() != "viewer" {
		t.Errorf("before.role = %v, want viewer", beforeValues["role"])
	}
	metadata := beforeValues["metadata"]
	if metadata.Type() != attribute.MAP || mapValues(metadata.AsMap())["approved"].AsBool() {
		t.Errorf("before.metadata = %v, want Map(approved=false)", metadata)
	}
	if mapValues(after.AsMap())["role"].AsString() != "admin" {
		t.Errorf("after.role = %v, want admin", mapValues(after.AsMap())["role"])
	}
}

func mapValues(kvs []attribute.KeyValue) map[string]attribute.Value {
	values := make(map[string]attribute.Value, len(kvs))
	for _, kv := range kvs {
		values[string(kv.Key)] = kv.Value
	}
	return values
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
		slog.String("service.name", "forged"),
		slog.String("service.version", "forged"),
		slog.String("service.instance.id", "forged"),
		slog.String("deployment.environment.name", "forged"),
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
		slog.String("app.result", "success"),
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
	if len(rest) != 1 || rest[0].Key != "app.result" {
		t.Fatalf("rest = %v, want 仅 app.result", rest)
	}
}
