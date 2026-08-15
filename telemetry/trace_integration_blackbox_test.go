package telemetry_test

import (
	"context"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/formal-you/go-observability/telemetry"
)

type retainingTraceExporter struct{ *tracetest.InMemoryExporter }

func (retainingTraceExporter) Shutdown(context.Context) error { return nil }

var _ trace.SpanExporter = retainingTraceExporter{}

func TestRuntimeTraceTreeBlackBox(t *testing.T) {
	exporter := retainingTraceExporter{InMemoryExporter: tracetest.NewInMemoryExporter()}
	runtime, err := telemetry.NewRuntime(context.Background(), telemetry.Config{
		Enabled:  true,
		Resource: telemetry.ResourceConfig{ServiceName: "trace-contract"},
		Trace: telemetry.TraceConfig{
			Output:       telemetry.SignalOutputOTLP,
			SampleRatio:  1,
			Exporter:     exporter,
			BatchTimeout: time.Hour,
		},
		Metric: telemetry.MetricConfig{
			Output:         telemetry.SignalOutputOTLP,
			Reader:         sdkmetric.NewManualReader(),
			ExportInterval: time.Hour,
		},
		Log: telemetry.LogConfig{Output: telemetry.SignalOutputNone},
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	tracer := runtime.Tracer("blackbox")
	ctx, root := tracer.Start(context.Background(), "http.request")
	_, child := tracer.Start(ctx, "db.query")
	child.End()
	root.End()
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("span count = %d, want 2", len(spans))
	}
	var rootSpan, childSpan *tracetest.SpanStub
	for i := range spans {
		span := &spans[i]
		if span.Parent.IsValid() {
			childSpan = span
		} else {
			rootSpan = span
		}
	}
	if rootSpan == nil || childSpan == nil {
		t.Fatal("expected one root span and one child span")
	}
	if childSpan.Parent.SpanID() != rootSpan.SpanContext.SpanID() {
		t.Fatalf("child parent = %s, root = %s", childSpan.Parent.SpanID(), rootSpan.SpanContext.SpanID())
	}
	if childSpan.SpanContext.TraceID() != rootSpan.SpanContext.TraceID() {
		t.Fatal("root and child must share trace id")
	}
}
