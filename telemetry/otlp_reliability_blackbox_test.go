package telemetry_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/formal-you/go-observability/telemetry"
)

type noOpTraceExporter struct{}

func (noOpTraceExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error { return nil }
func (noOpTraceExporter) Shutdown(context.Context) error                             { return nil }

func TestRuntimeOTLPUnavailableIsObservableBlackBox(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve endpoint: %v", err)
	}
	endpoint := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved endpoint: %v", err)
	}
	runtime, err := telemetry.NewRuntime(context.Background(), telemetry.Config{
		Enabled:              true,
		ServiceName:          "otlp-unavailable",
		Endpoint:             endpoint,
		LogOutput:            telemetry.LogOutputOTLP,
		LogBatchTimeout:      10 * time.Millisecond,
		TraceExporter:        noOpTraceExporter{},
		MetricReader:         sdkmetric.NewManualReader(),
		TraceBatchTimeout:    time.Hour,
		MetricExportInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	writer, err := runtime.NewWriter(context.Background(), telemetry.WriterConfig{})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := writer.Write(context.Background(), "error", slog.String("level", "ERROR")); err != nil {
		t.Fatalf("Write should enqueue without hiding exporter failure: %v", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = runtime.Shutdown(shutdownCtx)
	if stats := runtime.Stats(); stats.LogExportErrors == 0 {
		t.Fatal("collector/exporter failure must be observable through Runtime.Stats")
	}
}
