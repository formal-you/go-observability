package file

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterAppendsJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "events.jsonl")
	w, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())

	if err := w.Write(context.Background(), "access", slog.String("http.request.method", "GET"), slog.String("app.result", "success")); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "business", slog.String("event.name", "business.order.paid")); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("行数 = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"http.request.method":"GET"`) || !strings.Contains(lines[0], `"msg":"access"`) {
		t.Errorf("line0 = %s", lines[0])
	}
	if !strings.Contains(lines[1], `"event.name":"business.order.paid"`) {
		t.Errorf("line1 = %s", lines[1])
	}
}

// TestWriteCanonicalOrder 验证 JSONL 行的固定字段顺序：
// level → msg → 链路/延迟 → event.name → payload 字段 → app.result 收尾，
// 防止字段顺序退化成字典序或随构造顺序漂移。
func TestWriteCanonicalOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())

	// 模拟一次 access 事件的扁平 attrs（normalize 输出顺序：metadata → payload → app.result）。
	if err := w.Write(context.Background(), "access",
		slog.String("level", "INFO"),
		slog.String("trace_id", "949058d5c20153624d52da3358038026"),
		slog.String("span_id", "bba473e9b3034af6"),
		slog.String("request_id", "req-1001"),
		slog.Int64("latency_ms", 12),
		slog.String("event.name", "access.http.request"),
		slog.String("http.request.method", "GET"),
		slog.String("url.path", "/api/v1/products/42"),
		slog.Int("http.response.status_code", 200),
		slog.String("app.result", "success"),
	); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"level":"INFO","msg":"access","trace_id":"949058d5c20153624d52da3358038026","span_id":"bba473e9b3034af6","request_id":"req-1001","latency_ms":12,"event.name":"access.http.request","http.request.method":"GET","url.path":"/api/v1/products/42","http.response.status_code":200,"app.result":"success"}` + "\n"
	if got := string(data); got != want {
		t.Errorf("字段顺序不符：\n got=%s\nwant=%s", got, want)
	}
}
