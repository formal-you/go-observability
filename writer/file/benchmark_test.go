package file

import (
	"context"
	"log/slog"
	"testing"

	obslog "github.com/formal-you/go-observability/log"
)

func BenchmarkWriterWrite(b *testing.B) {
	w, err := New(b.TempDir()+"/events.jsonl", WithResourceMetadata(ResourceMetadata{
		ServiceName: "benchmark-service",
	}))
	if err != nil {
		b.Fatal(err)
	}
	defer w.Close(context.Background())
	attrs := []slog.Attr{
		slog.String(string(obslog.KeyTimestamp), "2026-08-11T12:00:00Z"),
		slog.String(string(obslog.KeyLevel), "INFO"),
		slog.String(string(obslog.KeyEventName), "order.payment.succeeded"),
		slog.String(string(obslog.KeyTraceID), "0123456789abcdef0123456789abcdef"),
		slog.String("app.order_id", "ORD-1001"),
		slog.Int64("app.amount_cents", 9900),
		slog.String(string(obslog.KeyAppResult), "success"),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := w.Write(context.Background(), "business", attrs...); err != nil {
			b.Fatal(err)
		}
	}
}
