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

	if err := w.Write(context.Background(), "access", slog.String("http.request.method", "GET"), slog.String("mall.result", "success")); err != nil {
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
