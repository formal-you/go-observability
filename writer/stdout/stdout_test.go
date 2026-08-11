package stdout

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestWriterEmitsToOutput(t *testing.T) {
	var buf bytes.Buffer
	w, err := New(context.Background(), WithOutput(&buf))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())
	if err := w.Write(context.Background(), "access"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "access") {
		t.Errorf("输出应包含事件类型: %s", out)
	}
}

// TestWriteStripsRecordFields 验证顶层字段映射与保留键剥离：SeverityText 出现在顶层，
// level/timestamp/trace_id/span_id 不再作为属性输出。
func TestWriteStripsRecordFields(t *testing.T) {
	var buf bytes.Buffer
	w, err := New(context.Background(), WithOutput(&buf))
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close(context.Background())

	ts := time.Date(2026, 8, 8, 21, 34, 56, 0, time.UTC)
	if err := w.Write(context.Background(), "access",
		slog.Time("timestamp", ts),
		slog.String("level", "WARN"),
		slog.String("event.name", "http.request.completed"),
		slog.String("trace_id", "949058d5c20153624d52da3358038026"),
		slog.String("span_id", "bba473e9b3034af6"),
		slog.String("http.request.method", "GET"),
	); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"SeverityText":"WARN"`) {
		t.Errorf("输出缺少 SeverityText=WARN: %s", out)
	}
	if !strings.Contains(out, `"EventName":"http.request.completed"`) {
		t.Errorf("输出缺少 EventName=http.request.completed: %s", out)
	}
	for _, reserved := range []string{`"Key":"level"`, `"Key":"timestamp"`, `"Key":"event.name"`, `"Key":"trace_id"`, `"Key":"span_id"`} {
		if strings.Contains(out, reserved) {
			t.Errorf("输出仍含保留属性 %s: %s", reserved, out)
		}
	}
	if !strings.Contains(out, `"Key":"http.request.method"`) {
		t.Errorf("输出缺 http.request.method: %s", out)
	}
}
