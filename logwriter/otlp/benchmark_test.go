package otlp

import (
	"context"
	"log/slog"
	"testing"
	"time"

	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type benchmarkExporter struct{}

func (benchmarkExporter) Export(context.Context, []sdklog.Record) error { return nil }
func (benchmarkExporter) Shutdown(context.Context) error                { return nil }
func (benchmarkExporter) ForceFlush(context.Context) error              { return nil }

func BenchmarkWriterEnqueue(b *testing.B) {
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(
		benchmarkExporter{}, sdklog.WithExportInterval(time.Hour), sdklog.WithMaxQueueSize(4096),
	)))
	w, err := New(context.Background(), WithLoggerProvider(provider))
	if err != nil {
		b.Fatal(err)
	}
	defer provider.Shutdown(context.Background())
	attrs := []slog.Attr{
		slog.String("timestamp", "2026-08-11T12:00:00Z"),
		slog.String("level", "INFO"),
		slog.String("event.name", "order.payment.succeeded"),
		slog.String("trace_id", "0123456789abcdef0123456789abcdef"),
		slog.String("app.order_id", "ORD-1001"),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := w.Write(context.Background(), "business", attrs...); err != nil {
			b.Fatal(err)
		}
	}
}
