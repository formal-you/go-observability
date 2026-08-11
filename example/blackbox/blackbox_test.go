package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/writer/file"
)

// TestBlackboxJSONL 黑盒断言：把 emitAll 的真实 JSON 输出逐行校验结构，
// 防止实现漂移（msg=event_type、字段键、堆栈有无、级别等）。
func TestBlackboxJSONL(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := file.New(path)
	if err != nil {
		t.Fatal(err)
	}
	emitAll(ctx, log.NewLogger(w))
	if err := w.Close(ctx); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 9 {
		t.Fatalf("want 9 event lines, got %d", len(lines))
	}

	decode := func(i int) map[string]any {
		t.Helper()
		m := map[string]any{}
		if err := json.Unmarshal([]byte(lines[i]), &m); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", i+1, err)
		}
		return m
	}
	assertStr := func(m map[string]any, key, want string) {
		t.Helper()
		if got, ok := m[key].(string); !ok || got != want {
			t.Errorf("line key %q = %v, want %q", key, m[key], want)
		}
	}
	assertNum := func(m map[string]any, key string, want float64) {
		t.Helper()
		if got, ok := m[key].(float64); !ok || got != want {
			t.Errorf("line key %q = %v, want %v", key, m[key], want)
		}
	}
	assertStack := func(m map[string]any, key string, want bool) {
		t.Helper()
		v, ok := m[key].(string)
		if want && (!ok || v == "") {
			t.Errorf("line key %q should contain a stack trace, got absent/empty", key)
		}
		if !want && ok && v != "" {
			t.Errorf("line key %q should be omitted (no stack), got %q", key, v)
		}
	}

	// 0: access 200
	m := decode(0)
	assertStr(m, "msg", "access")
	assertStr(m, "event.name", "access.http.request")
	assertStr(m, "level", "INFO")
	assertNum(m, "http.response.status_code", 200)
	assertStr(m, "app.result", "success")

	// 1: access 404
	m = decode(1)
	assertStr(m, "msg", "access")
	assertStr(m, "level", "WARN")
	assertNum(m, "http.response.status_code", 404)
	assertStr(m, "app.result", "failed")

	// 2: business success
	m = decode(2)
	assertStr(m, "msg", "business")
	assertStr(m, "event.name", "business.order.paid")
	assertStr(m, "app.result", "success")
	assertStr(m, "app.order_id", "ORD-20260811-0001")

	// 3: business rejection（库存不足）
	m = decode(3)
	assertStr(m, "msg", "business")
	assertStr(m, "error.type", "business.order.stock_insufficient")
	assertStr(m, "app.business_code", "ORDER.CREATE.STOCK_INSUFFICIENT")
	assertStr(m, "app.business_message", "商品库存不足")
	assertStr(m, "app.result", "blocked")

	// 4: validation（errs 投影）
	m = decode(4)
	assertStr(m, "msg", "business")
	assertStr(m, "event.name", "business.user.register")
	assertStr(m, "error.type", "validation.failed")
	assertStr(m, "app.result", "failed")

	// 5: 非预期系统错误——DB 连接失败：堆栈必须有，可重试未耗尽 → WARN
	m = decode(5)
	assertStr(m, "msg", "error")
	assertStr(m, "event.name", "error.db.connection")
	assertStr(m, "error.type", "db.connection_error")
	assertStr(m, "exception.message", "dial tcp 127.0.0.1:5432: connect: connection refused")
	assertStr(m, "app.upstream_service", "postgres")
	assertStr(m, "level", "WARN")
	assertStack(m, "exception.stacktrace", true)

	// 6: MQ 发布失败：堆栈必须有，重试耗尽 → ERROR
	m = decode(6)
	assertStr(m, "msg", "error")
	assertStr(m, "error.type", "mq.publish_failed")
	assertStr(m, "level", "ERROR")
	assertNum(m, "app.retry_count", 3)
	assertStack(m, "exception.stacktrace", true)

	// 7: runtime panic：堆栈必须有
	m = decode(7)
	assertStr(m, "msg", "error")
	assertStr(m, "event.name", "error.runtime.panic")
	assertStr(m, "error.type", "runtime.panic")
	assertStr(m, "level", "ERROR")
	assertStack(m, "exception.stacktrace", true)

	// 8: lock conflict：StackOptional → 无堆栈（对比项）
	m = decode(8)
	assertStr(m, "msg", "error")
	assertStr(m, "error.type", "lock.conflict")
	assertStr(m, "level", "ERROR")
	assertStack(m, "exception.stacktrace", false)
}

// orderedKeys 按 JSON 对象实际写入顺序返回键名（不做字典序），用于锁定规范字段顺序。
func orderedKeys(t *testing.T, line string) []string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(line))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("decode first token: %v", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		t.Fatalf("line 不是 JSON 对象: %v", tok)
	}
	var keys []string
	for dec.More() {
		ktok, err := dec.Token()
		if err != nil {
			t.Fatalf("decode key: %v", err)
		}
		keys = append(keys, ktok.(string))
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("decode value: %v", err)
		}
	}
	return keys
}

// TestBlackboxJSONLCanonicalOrder 断言 9 条事件共享同一规范字段顺序：
// msg 恒紧跟 level（事件粗分类），末键恒为 app.result（结果收尾）；
// 代表行锁定完整键序，防止实现漂移（与 writer/file 的固定顺序契约一致）。
func TestBlackboxJSONLCanonicalOrder(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := file.New(path)
	if err != nil {
		t.Fatal(err)
	}
	emitAll(ctx, log.NewLogger(w))
	if err := w.Close(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 9 {
		t.Fatalf("want 9 event lines, got %d", len(lines))
	}

	for i, line := range lines {
		keys := orderedKeys(t, line)
		if keys[1] != "msg" {
			t.Errorf("line %d: msg 未紧跟 level（level=%q, next=%q），want 固定顺序 level→msg", i+1, keys[0], keys[1])
		}
		if keys[len(keys)-1] != "app.result" {
			t.Errorf("line %d 末键 = %q, want app.result（结果收尾）", i+1, keys[len(keys)-1])
		}
	}

	exact := map[int][]string{
		0: {"level", "msg", "trace_id", "span_id", "request_id", "latency_ms", "event.name",
			"http.request.method", "url.path", "http.route", "http.response.status_code",
			"client.address", "user_agent.original", "app.result"},
		2: {"level", "msg", "event.name", "app.order_id", "app.amount", "app.result"},
		5: {"level", "msg", "event.name", "error.type", "exception.message", "exception.stacktrace",
			"app.retryable", "app.retry_count", "app.upstream_service", "app.result"},
		8: {"level", "msg", "event.name", "error.type", "exception.message", "app.retryable", "app.result"},
	}
	for i, want := range exact {
		if got := orderedKeys(t, lines[i]); !slices.Equal(got, want) {
			t.Errorf("line %d 键序 = %v, want %v", i+1, got, want)
		}
	}
}
