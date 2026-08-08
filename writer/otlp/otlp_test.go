package otlp

import (
	"context"
	"log/slog"
	"testing"
)

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
