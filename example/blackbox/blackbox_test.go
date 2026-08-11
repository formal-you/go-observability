package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	otelog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/otelutil"
	otlpwriter "github.com/formal-you/go-observability/writer/otlp"
)

const blackboxEventCount = 12

// TestBlackboxJSONL 验证真实 HTTP/后台场景的扁平运营投影与跨事件关联。
func TestBlackboxJSONL(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if _, err := writeFileSample(ctx, path); err != nil {
		t.Fatal(err)
	}
	// 第二次运行必须覆盖旧样例，而不是追加另一批事件。
	report, err := writeFileSample(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	events, lines := readJSONLEvents(t, path)
	if len(events) != blackboxEventCount {
		t.Fatalf("event lines = %d, want %d", len(events), blackboxEventCount)
	}

	assertRequestScenario(t, events, report, requestBusinessSuccess,
		[]string{"business", "access"}, []string{"business.order.paid", "access.http.request"})
	assertRequestScenario(t, events, report, requestBusinessFailed,
		[]string{"business", "access"}, []string{"error.http.request", "access.http.request"})
	assertRequestScenario(t, events, report, requestSystemError,
		[]string{"error", "security", "audit", "access"},
		[]string{"error.http.request", "security.input.anomaly", "audit.input.anomaly", "access.http.request"})
	assertRequestScenario(t, events, report, requestPanic,
		[]string{"error", "access"}, []string{"error.runtime.panic", "access.http.request"})
	assertBackgroundScenarios(t, events, report)
	assertDistinctRequestTraces(t, report)
	assertCanonicalOrder(t, lines)
}

func assertRequestScenario(
	t *testing.T,
	events []map[string]any,
	report *scenarioReport,
	requestID string,
	wantTypes []string,
	wantNames []string,
) {
	t.Helper()
	got := eventsForRequest(events, requestID)
	if len(got) != len(wantTypes) {
		t.Fatalf("request_id=%s events=%d, want %d: %v", requestID, len(got), len(wantTypes), got)
	}
	traceInfo, ok := report.snapshot()[requestID]
	if !ok {
		t.Fatalf("request_id=%s 缺场景 trace", requestID)
	}
	assertValidTrace(t, traceInfo)
	for i, event := range got {
		assertString(t, event, "msg", wantTypes[i])
		assertString(t, event, "event.name", wantNames[i])
		assertString(t, event, "trace_id", traceInfo.TraceID)
		assertString(t, event, "span_id", traceInfo.SpanID)
		assertString(t, event, "request_id", requestID)
		if event["msg"] == "access" {
			assertAccessShape(t, event)
		} else {
			assertNoHTTPFields(t, event)
		}
	}
}

func assertAccessShape(t *testing.T, event map[string]any) {
	t.Helper()
	for _, key := range []string{"http.request.method", "url.path", "http.route", "http.response.status_code", "latency_ms"} {
		if _, ok := event[key]; !ok {
			t.Errorf("access 缺少 %s: %v", key, event)
		}
	}
	if latency, ok := event["latency_ms"].(float64); !ok || latency <= 0 {
		t.Errorf("latency_ms = %v, want > 0", event["latency_ms"])
	}
}

func assertNoHTTPFields(t *testing.T, event map[string]any) {
	t.Helper()
	for _, key := range []string{"http.request.method", "url.path", "http.route", "http.response.status_code", "latency_ms"} {
		if _, ok := event[key]; ok {
			t.Errorf("%s 事件不应重复 access 字段 %s", event["msg"], key)
		}
	}
}

func assertBackgroundScenarios(t *testing.T, events []map[string]any, report *scenarioReport) {
	t.Helper()
	traces := report.snapshot()
	for _, item := range []struct {
		scenario  string
		eventName string
		wantStack bool
	}{
		{"background-mq", "error.mq.publish", true},
		{"background-lock", "error.lock.conflict", false},
	} {
		var found map[string]any
		for _, event := range events {
			if event["event.name"] == item.eventName {
				found = event
				break
			}
		}
		if found == nil {
			t.Fatalf("缺少后台事件 %s", item.eventName)
		}
		if _, ok := found["request_id"]; ok {
			t.Errorf("后台事件 %s 不应伪造 request_id", item.eventName)
		}
		assertNoHTTPFields(t, found)
		traceInfo := traces[item.scenario]
		assertValidTrace(t, traceInfo)
		assertString(t, found, "trace_id", traceInfo.TraceID)
		assertString(t, found, "span_id", traceInfo.SpanID)
		stack, hasStack := found["exception.stacktrace"].(string)
		if item.wantStack && (!hasStack || stack == "") {
			t.Errorf("%s 应包含 stacktrace", item.eventName)
		}
		if !item.wantStack && hasStack && stack != "" {
			t.Errorf("%s 不应包含 stacktrace", item.eventName)
		}
	}
}

func assertDistinctRequestTraces(t *testing.T, report *scenarioReport) {
	t.Helper()
	seen := map[string]string{}
	for _, requestID := range []string{requestBusinessSuccess, requestBusinessFailed, requestSystemError, requestPanic} {
		traceID := report.snapshot()[requestID].TraceID
		if previous, exists := seen[traceID]; exists {
			t.Errorf("request %s 与 %s 复用了 trace_id=%s", requestID, previous, traceID)
		}
		seen[traceID] = requestID
	}
}

func assertValidTrace(t *testing.T, got scenarioTrace) {
	t.Helper()
	tid, err := trace.TraceIDFromHex(got.TraceID)
	if err != nil || !tid.IsValid() {
		t.Errorf("trace_id=%q 不是有效 OTel TraceID: %v", got.TraceID, err)
	}
	sid, err := trace.SpanIDFromHex(got.SpanID)
	if err != nil || !sid.IsValid() {
		t.Errorf("span_id=%q 不是有效 OTel SpanID: %v", got.SpanID, err)
	}
}

func eventsForRequest(events []map[string]any, requestID string) []map[string]any {
	var out []map[string]any
	for _, event := range events {
		if event["request_id"] == requestID {
			out = append(out, event)
		}
	}
	return out
}

func assertString(t *testing.T, event map[string]any, key, want string) {
	t.Helper()
	if got, ok := event[key].(string); !ok || got != want {
		t.Errorf("%s = %v, want %q", key, event[key], want)
	}
}

func readJSONLEvents(t *testing.T, path string) ([]map[string]any, []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	events := make([]map[string]any, 0, len(lines))
	for i, line := range lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("line %d 不是合法 JSON: %v", i+1, err)
		}
		events = append(events, event)
	}
	return events, lines
}

// orderedKeys 按 JSON 对象实际写入顺序返回键名，用于锁定 file Writer 运营投影。
func orderedKeys(t *testing.T, line string) []string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(line))
	tok, err := dec.Token()
	if err != nil {
		t.Fatal(err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		t.Fatalf("line 不是 JSON 对象: %v", tok)
	}
	var keys []string
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key.(string))
		var value any
		if err := dec.Decode(&value); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

func assertCanonicalOrder(t *testing.T, lines []string) {
	t.Helper()
	for i, line := range lines {
		keys := orderedKeys(t, line)
		levelIndex := slices.Index(keys, "level")
		msgIndex := slices.Index(keys, "msg")
		if levelIndex < 0 || msgIndex != levelIndex+1 {
			t.Errorf("line %d keys=%v, want level 紧邻 msg", i+1, keys)
		}
		if keys[len(keys)-1] != "app.result" {
			t.Errorf("line %d 末键=%q, want app.result", i+1, keys[len(keys)-1])
		}
	}
}

// captureProcessor 捕获 SDK LogRecord，验证 OTLP 顶层字段而不依赖 Collector。
type captureProcessor struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (*captureProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }

func (p *captureProcessor) OnEmit(_ context.Context, record *sdklog.Record) error {
	p.mu.Lock()
	p.records = append(p.records, record.Clone())
	p.mu.Unlock()
	return nil
}

func (*captureProcessor) Shutdown(context.Context) error   { return nil }
func (*captureProcessor) ForceFlush(context.Context) error { return nil }

func (p *captureProcessor) snapshot() []sdklog.Record {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.records)
}

// TestBlackboxOTLPSemantics 验证同一批真实场景在 OTLP 中映射到正确的顶层字段。
func TestBlackboxOTLPSemantics(t *testing.T) {
	ctx := context.Background()
	processor := &captureProcessor{}
	logProvider := sdklog.NewLoggerProvider(sdklog.WithProcessor(processor))
	writer, err := otlpwriter.New(ctx, otlpwriter.WithLoggerProvider(logProvider))
	if err != nil {
		t.Fatal(err)
	}
	traceProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	logger := log.NewLogger(writer, log.WithTraceExtractor(otelutil.NewTraceExtractor()))
	report, emitErr := emitAll(ctx, logger, traceProvider.Tracer(blackboxServiceName))
	closeErr := writer.Close(ctx)
	traceErr := traceProvider.Shutdown(ctx)
	logErr := logProvider.Shutdown(ctx)
	for _, err := range []error{emitErr, closeErr, traceErr, logErr} {
		if err != nil {
			t.Fatal(err)
		}
	}

	records := processor.snapshot()
	if len(records) != blackboxEventCount {
		t.Fatalf("OTLP records=%d, want %d", len(records), blackboxEventCount)
	}
	traces := report.snapshot()
	for _, record := range records {
		attrs := recordAttributes(&record)
		for _, reserved := range []string{"timestamp", "level", "event.name", "trace_id", "span_id"} {
			if _, ok := attrs[reserved]; ok {
				t.Errorf("OTLP Attributes 不应包含保留键 %s", reserved)
			}
		}
		if record.EventName() == "" {
			t.Error("OTLP EventName 不能为空")
		}
		if record.Body().AsString() == "" {
			t.Error("OTLP Body(event_type) 不能为空")
		}
		if requestID, ok := attributeString(attrs["request_id"]); ok {
			want := traces[requestID]
			if record.TraceID().String() != want.TraceID || record.SpanID().String() != want.SpanID {
				t.Errorf("request_id=%s OTLP trace/span=%s/%s, want %s/%s",
					requestID, record.TraceID(), record.SpanID(), want.TraceID, want.SpanID)
			}
			if record.Body().AsString() == "access" {
				if _, ok := attrs["http.response.status_code"]; !ok {
					t.Errorf("request_id=%s access 缺 http.response.status_code", requestID)
				}
			} else {
				for _, key := range []string{"http.request.method", "url.path", "http.response.status_code", "latency_ms"} {
					if _, ok := attrs[key]; ok {
						t.Errorf("request_id=%s %s 不应含 %s", requestID, record.Body().AsString(), key)
					}
				}
			}
		}
	}

	panicAccess := findOTLPRecord(records, requestPanic, "access")
	if panicAccess == nil {
		t.Fatal("缺少 panic 请求的 OTLP AccessEvent")
	}
	if panicAccess.Severity() != otelog.SeverityError || panicAccess.EventName() != "access.http.request" {
		t.Errorf("panic access severity/event=%v/%q, want ERROR/access.http.request",
			panicAccess.Severity(), panicAccess.EventName())
	}
}

func recordAttributes(record *sdklog.Record) map[string]attribute.Value {
	out := map[string]attribute.Value{}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		out[string(kv.Key)] = kv.Value
		return true
	})
	return out
}

func attributeString(value attribute.Value) (string, bool) {
	if value.Type() != attribute.STRING {
		return "", false
	}
	return value.AsString(), true
}

func findOTLPRecord(records []sdklog.Record, requestID, body string) *sdklog.Record {
	for i := range records {
		attrs := recordAttributes(&records[i])
		gotRequestID, _ := attributeString(attrs["request_id"])
		if gotRequestID == requestID && records[i].Body().AsString() == body {
			return &records[i]
		}
	}
	return nil
}
