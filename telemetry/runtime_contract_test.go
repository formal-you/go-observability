package telemetry_test

import (
	"context"
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
	for _, output := range []telemetry.LogOutput{
		telemetry.LogOutputFile,
		telemetry.LogOutputStdout,
		telemetry.LogOutputNone,
	} {
		r, err := telemetry.NewRuntime(ctx, telemetry.Config{
			Enabled: true, ServiceName: "runtime-contract", LogOutput: output,
			TraceBatchTimeout: time.Hour, MetricExportInterval: time.Hour,
		})
		if err != nil {
			t.Fatalf("NewRuntime(%q): %v", output, err)
		}
		if r.LoggerProvider() != nil {
			t.Fatalf("output %q must not create an OTLP LoggerProvider", output)
		}
		_ = r.Shutdown(ctx)
	}

	r, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled: true, ServiceName: "runtime-contract", LogOutput: telemetry.LogOutputOTLP,
		TraceBatchTimeout: time.Hour, MetricExportInterval: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(ctx)
	if r.LoggerProvider() == nil {
		t.Fatal("otlp output must create a LoggerProvider")
	}
}

func TestRuntimeInstallGlobalRestoresBlackBox(t *testing.T) {
	ctx := context.Background()
	oldTrace := otel.GetTracerProvider()
	oldMeter := otel.GetMeterProvider()
	oldLogger := global.GetLoggerProvider()
	oldPropagator := otel.GetTextMapPropagator()

	r, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled: true, ServiceName: "runtime-contract", LogOutput: telemetry.LogOutputNone,
		TraceBatchTimeout: time.Hour, MetricExportInterval: time.Hour,
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
	r, err := telemetry.NewFileRuntime(telemetry.Config{ServiceName: "file-only"})
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
