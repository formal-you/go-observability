package otlp_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/formal-you/go-observability/logwriter/otlp"
)

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range records {
		e.records = append(e.records, records[i].Clone())
	}
	return nil
}

func (*recordingExporter) Shutdown(context.Context) error   { return nil }
func (*recordingExporter) ForceFlush(context.Context) error { return nil }

func TestBatchShutdownFlushesBlackBox(t *testing.T) {
	exporter := &recordingExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(
		exporter,
		sdklog.WithExportInterval(time.Hour),
		sdklog.WithMaxQueueSize(16),
	)))
	writer, err := otlp.New(context.Background(), otlp.WithLoggerProvider(provider))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := writer.Write(context.Background(), "business",
		slog.String("level", "INFO"),
		slog.String("event.name", "order.payment.succeeded"),
	); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	if len(exporter.records) != 1 {
		t.Fatalf("exported records = %d, want 1", len(exporter.records))
	}
}
