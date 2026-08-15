package telemetry_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"

	"github.com/formal-you/go-observability/telemetry"
)

func TestNewRuntimeOutputContractBlackBox(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  true,
		Resource: telemetry.ResourceConfig{ServiceName: "runtime-contract"},
		Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputNone},
		Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputNone},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputOTLP},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(ctx)
	w, err := r.NewWriter(ctx)
	if err != nil {
		t.Fatalf("OTLP runtime must create its configured Writer: %v", err)
	}
	if err := w.Close(ctx); err != nil {
		t.Fatalf("close runtime-backed OTLP Writer: %v", err)
	}
}

func TestRuntimeInstallGlobalRestoresBlackBox(t *testing.T) {
	ctx := context.Background()
	oldTrace := otel.GetTracerProvider()
	oldMeter := otel.GetMeterProvider()
	oldLogger := global.GetLoggerProvider()
	oldPropagator := otel.GetTextMapPropagator()

	r, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  true,
		Resource: telemetry.ResourceConfig{ServiceName: "runtime-contract"},
		Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputOTLP, BatchTimeout: time.Hour},
		Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputOTLP, ExportInterval: time.Hour},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputNone},
	})
	if err != nil {
		t.Fatal(err)
	}
	restore := r.InstallGlobal()
	if otel.GetTracerProvider() == oldTrace || otel.GetMeterProvider() == oldMeter ||
		global.GetLoggerProvider() != oldLogger || reflect.DeepEqual(otel.GetTextMapPropagator(), oldPropagator) {
		t.Fatal("InstallGlobal did not install the runtime providers and propagator")
	}
	restore()
	restore()
	if otel.GetTracerProvider() != oldTrace || otel.GetMeterProvider() != oldMeter ||
		global.GetLoggerProvider() != oldLogger || !reflect.DeepEqual(otel.GetTextMapPropagator(), oldPropagator) {
		t.Fatal("restore did not return the original global objects")
	}
	_ = r.Shutdown(ctx)
}

func TestNewFileRuntimeCreatesUnsampledTraceIDsBlackBox(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	r, err := telemetry.NewFileRuntime(telemetry.Config{
		Resource: telemetry.ResourceConfig{ServiceName: "file-only"},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(context.Background())
	tracer := r.Tracer("blackbox")
	_, first := tracer.Start(context.Background(), "first")
	_, second := tracer.Start(context.Background(), "second")
	defer first.End()
	defer second.End()
	if !first.SpanContext().TraceID().IsValid() || !first.SpanContext().SpanID().IsValid() || first.SpanContext().IsSampled() {
		t.Fatal("first file-only span must have valid non-sampled IDs")
	}
	if !second.SpanContext().TraceID().IsValid() || first.SpanContext().TraceID() == second.SpanContext().TraceID() {
		t.Fatal("independent file-only root spans must have distinct trace IDs")
	}
}

func TestNewRuntimeSkipsEndpointValidationBlackBox(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	r, err := telemetry.NewRuntime(context.Background(), telemetry.Config{
		Enabled:  true,
		Resource: telemetry.ResourceConfig{ServiceName: "local"},
		Endpoint: "collector",
		Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputLocal},
		Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputNone},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: path},
	})
	if err != nil {
		t.Fatalf("non-OTLP runtime must not parse endpoint: %v", err)
	}
	defer r.Shutdown(context.Background())
}
