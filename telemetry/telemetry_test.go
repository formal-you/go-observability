package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	stdoutwriter "github.com/formal-you/go-observability/writer/stdout"
)

func TestNewRuntimeDoesNotInstallGlobals(t *testing.T) {
	oldTrace := otel.GetTracerProvider()
	oldMeter := otel.GetMeterProvider()
	oldLogger := global.GetLoggerProvider()
	oldPropagator := otel.GetTextMapPropagator()
	r, err := NewRuntime(context.Background(), Config{
		Enabled:  true,
		Resource: ResourceConfig{ServiceName: "svc"},
		Trace:    TraceConfig{Output: SignalOutputNone},
		Metric:   MetricConfig{Output: SignalOutputNone},
		Log:      LogConfig{Output: SignalOutputNone},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(context.Background())
	if otel.GetTracerProvider() != oldTrace || otel.GetMeterProvider() != oldMeter || global.GetLoggerProvider() != oldLogger || !reflect.DeepEqual(otel.GetTextMapPropagator(), oldPropagator) {
		t.Fatal("NewRuntime must not modify OpenTelemetry globals")
	}
}

func TestNewRuntimeValidation(t *testing.T) {
	tests := []Config{
		{Enabled: true, Log: LogConfig{Output: SignalOutputFile}},
		{Enabled: true, Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputFile}},
		{Enabled: true, Resource: ResourceConfig{ServiceName: "svc"}},
		{Enabled: true, Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutput("invalid")}},
		{Enabled: true, Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputNone}, Trace: TraceConfig{Output: SignalOutput("invalid")}},
		{Enabled: true, Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputNone}, Metric: MetricConfig{Output: SignalOutputLocal}},
	}
	for _, cfg := range tests {
		if _, err := NewRuntime(context.Background(), cfg); err == nil {
			t.Errorf("NewRuntime(%+v) error = nil", cfg)
		}
	}
	path := filepath.Join(t.TempDir(), "disabled.jsonl")
	r, err := NewRuntime(context.Background(), Config{Enabled: false, Log: LogConfig{Output: SignalOutputFile, FilePath: path}})
	if err != nil || r == nil || r.resource != nil {
		t.Fatalf("disabled runtime = %#v, %v", r, err)
	}
}

func TestInstallGlobalRestoresInLIFOOrder(t *testing.T) {
	oldTrace := otel.GetTracerProvider()
	oldMeter := otel.GetMeterProvider()
	oldLogger := global.GetLoggerProvider()
	oldPropagator := otel.GetTextMapPropagator()
	baseTrace := sdktrace.NewTracerProvider()
	baseMeter := sdkmetric.NewMeterProvider()
	baseLogger := sdklog.NewLoggerProvider()
	basePropagator := propagation.TraceContext{}
	otel.SetTracerProvider(baseTrace)
	otel.SetMeterProvider(baseMeter)
	global.SetLoggerProvider(baseLogger)
	otel.SetTextMapPropagator(basePropagator)
	t.Cleanup(func() {
		global.SetLoggerProvider(oldLogger)
		otel.SetMeterProvider(oldMeter)
		otel.SetTracerProvider(oldTrace)
		otel.SetTextMapPropagator(oldPropagator)
		_ = baseLogger.Shutdown(context.Background())
		_ = baseMeter.Shutdown(context.Background())
		_ = baseTrace.Shutdown(context.Background())
	})

	newRuntime := func(name string) (*Runtime, error) {
		return NewRuntime(context.Background(), Config{
			Enabled:  true,
			Resource: ResourceConfig{ServiceName: name},
			Trace:    TraceConfig{Output: SignalOutputOTLP, BatchTimeout: time.Hour},
			Metric:   MetricConfig{Output: SignalOutputOTLP, ExportInterval: time.Hour},
			Log:      LogConfig{Output: SignalOutputNone},
		})
	}
	r1, err := newRuntime("one")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := newRuntime("two")
	if err != nil {
		t.Fatal(err)
	}
	restore1 := r1.InstallGlobal()
	if otel.GetTracerProvider() != r1.tracerProvider || global.GetLoggerProvider() != baseLogger {
		t.Fatalf("first runtime was not installed: got trace=%T/%p want=%T/%p, logger got=%T want=%T", otel.GetTracerProvider(), otel.GetTracerProvider(), r1.tracerProvider, r1.tracerProvider, global.GetLoggerProvider(), baseLogger)
	}
	restore2 := r2.InstallGlobal()
	restore2()
	restore2()
	if otel.GetTracerProvider() != r1.tracerProvider || otel.GetMeterProvider() != r1.meterProvider || global.GetLoggerProvider() != baseLogger {
		t.Fatal("second restore did not return to first runtime")
	}
	restore1()
	if otel.GetTracerProvider() != baseTrace || otel.GetMeterProvider() != baseMeter || global.GetLoggerProvider() != baseLogger || otel.GetTextMapPropagator() != basePropagator {
		t.Fatal("first restore did not return to base globals")
	}
	_ = r2.Shutdown(context.Background())
	_ = r1.Shutdown(context.Background())
}

func TestNewWriterFileInjectsResourceMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	r, err := NewFileRuntime(Config{
		Resource: ResourceConfig{ServiceName: "mall", ServiceVersion: "1.2.3"},
		Log:      LogConfig{Output: SignalOutputFile, FilePath: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(context.Background())
	w, err := r.NewWriter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "business", slog.String("event.name", "order.payment.succeeded")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &got); err != nil {
		t.Fatal(err)
	}
	if got["service.name"] != "mall" || got["service.version"] != "1.2.3" {
		t.Fatalf("resource metadata = %v", got)
	}
}

func TestNewWriterStdoutUsesResourceAndOutput(t *testing.T) {
	r := &Runtime{
		resource:  resourceForConfig(ResourceConfig{ServiceName: "stdout-svc", Environment: "test"}),
		logConfig: LogConfig{Output: SignalOutputStdout, StdoutOptions: []stdoutwriter.Option{stdoutwriter.WithOutput(&bytes.Buffer{})}},
	}
	var buf bytes.Buffer
	r.logConfig.StdoutOptions = []stdoutwriter.Option{stdoutwriter.WithOutput(&buf)}
	w, err := r.NewWriter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "access"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "stdout-svc") || !strings.Contains(out, "access") {
		t.Fatalf("stdout output missing resource or event: %s", out)
	}
}

func TestNewWriterNoneIsNoop(t *testing.T) {
	w, err := (&Runtime{logConfig: LogConfig{Output: SignalOutputNone}}).NewWriter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "ignored", slog.String("key", "value")); err != nil {
		t.Fatal(err)
	}
}

func TestNewRuntimeDisabledKeepsFileWriter(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "disabled.jsonl")
	p, err := NewRuntime(ctx, Config{Enabled: false, Log: LogConfig{Output: SignalOutputFile, FilePath: path}})
	if err != nil {
		t.Fatalf("disabled runtime: %v", err)
	}
	if p == nil || p.resource != nil || p.loggerProvider != nil {
		t.Fatalf("disabled runtime should have no providers, got %+v", p)
	}
	if p.Tracer("x") == nil || p.Meter("x") == nil {
		t.Error("disabled runtime Tracer/Meter should fall back to global no-op")
	}
	w, err := p.NewWriter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(ctx, "business"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("disabled Shutdown should return nil, got %v", err)
	}
}

func TestNewRuntimeDisabledRequiresFilePath(t *testing.T) {
	if _, err := NewRuntime(context.Background(), Config{Enabled: false}); err == nil {
		t.Fatal("disabled runtime with file fallback must require file path")
	}
}

func TestNewRuntimeRequiresServiceName(t *testing.T) {
	_, err := NewRuntime(context.Background(), Config{Enabled: true, Log: LogConfig{Output: SignalOutputNone}})
	if err == nil {
		t.Fatal("missing service.name should error")
	}
}

func TestNewRuntimeInvalidSampleRatio(t *testing.T) {
	for _, ratio := range []float64{-0.1, 1.5} {
		_, err := NewRuntime(context.Background(), Config{
			Enabled:  true,
			Resource: ResourceConfig{ServiceName: "svc"},
			Trace:    TraceConfig{Output: SignalOutputOTLP, SampleRatio: ratio},
			Log:      LogConfig{Output: SignalOutputNone},
		})
		if err == nil {
			t.Errorf("ratio=%v should error", ratio)
		}
	}
}

func TestNewRuntimeResourceAttributes(t *testing.T) {
	ctx := context.Background()
	p, err := NewRuntime(ctx, Config{
		Enabled:  true,
		Endpoint: "127.0.0.1:4317",
		Resource: ResourceConfig{
			ServiceName: "svc", ServiceVersion: "1.2.3", Environment: "test", Region: "cn", Instance: "i-1",
		},
		Trace:  TraceConfig{Output: SignalOutputOTLP},
		Metric: MetricConfig{Output: SignalOutputOTLP},
		Log:    LogConfig{Output: SignalOutputNone},
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()
	if p.resource == nil || p.loggerProvider != nil {
		t.Fatal("enabled runtime should build resource without log provider")
	}

	want := map[string]string{
		"service.name":                "svc",
		"service.version":             "1.2.3",
		"deployment.environment.name": "test",
		"region":                      "cn",
		"service.instance.id":         "i-1",
	}
	got := map[string]string{}
	for _, kv := range p.resource.Attributes() {
		got[string(kv.Key)] = kv.Value.AsString()
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("resource 属性 %s = %q, want %q（全部: %v）", k, got[k], v, got)
		}
	}
}

func TestNewFileRuntimeCreatesLocalTracesAndMetadata(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	p, err := NewFileRuntime(Config{
		Resource: ResourceConfig{ServiceName: "mall-monolith", ServiceVersion: "1.0.0", Instance: "shop-01"},
		Log:      LogConfig{Output: SignalOutputFile, FilePath: path},
	})
	if err != nil {
		t.Fatalf("NewFileRuntime: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()
	if p.loggerProvider != nil {
		t.Fatal("NewFileRuntime should not create OTLP LoggerProvider")
	}
	attrs := map[string]string{}
	for _, kv := range p.resource.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	if attrs["service.name"] != "mall-monolith" || attrs["service.version"] != "1.0.0" || attrs["service.instance.id"] != "shop-01" || attrs["deployment.environment.name"] != "development" {
		t.Fatalf("NewFileRuntime Resource = %v", attrs)
	}
	tracer := p.Tracer("test")
	_, span1 := tracer.Start(ctx, "one")
	_, span2 := tracer.Start(ctx, "two")
	if !span1.SpanContext().TraceID().IsValid() || !span1.SpanContext().SpanID().IsValid() {
		t.Fatal("local span1 ID invalid")
	}
	if span1.SpanContext().IsSampled() {
		t.Fatal("file-only root span should not set sampled flag")
	}
	if !span2.SpanContext().TraceID().IsValid() || span1.SpanContext().TraceID() == span2.SpanContext().TraceID() {
		t.Fatal("independent local root spans should have distinct valid TraceID")
	}
	span1.End()
	span2.End()
}

func TestNewFileRuntimeRequiresServiceName(t *testing.T) {
	if _, err := NewFileRuntime(Config{}); err == nil {
		t.Fatal("NewFileRuntime missing service.name should error")
	}
}

func TestNewFileRuntimeRejectsConflictingOutputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	tests := []Config{
		{Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputFile, FilePath: path}, Trace: TraceConfig{Output: SignalOutputOTLP}},
		{Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputFile, FilePath: path}, Metric: MetricConfig{Output: SignalOutputOTLP}},
		{Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputStdout}},
	}
	for _, cfg := range tests {
		if _, err := NewFileRuntime(cfg); err == nil {
			t.Errorf("NewFileRuntime(%+v) should reject conflicting output", cfg)
		}
	}
}

func TestShutdownNilAndDisabled(t *testing.T) {
	ctx := context.Background()
	if err := (*Runtime)(nil).Shutdown(ctx); err != nil {
		t.Errorf("nil Shutdown should return nil, got %v", err)
	}
	if err := (&Runtime{}).Shutdown(ctx); err != nil {
		t.Errorf("empty Runtime Shutdown should return nil, got %v", err)
	}
}

type failingLogProcessor struct{}

func (failingLogProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool {
	return true
}

func (failingLogProcessor) OnEmit(context.Context, *sdklog.Record) error {
	return nil
}

func (failingLogProcessor) Shutdown(context.Context) error {
	return context.DeadlineExceeded
}

func (failingLogProcessor) ForceFlush(context.Context) error {
	return nil
}

func TestShutdownRecordsLoggerProviderFailure(t *testing.T) {
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(failingLogProcessor{}))
	runtime := &Runtime{
		loggerProvider: provider,
		counters:       new(runtimeCounters),
	}

	if err := runtime.Shutdown(context.Background()); err == nil {
		t.Fatal("Shutdown should return the logger provider failure")
	}
	if stats := runtime.Stats(); stats.LogExportErrors != 1 {
		t.Fatalf("logger provider shutdown failure count = %d, want 1", stats.LogExportErrors)
	}
}

func TestEnabledFromEnvironment(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	if EnabledFromEnvironment() {
		t.Error("OTEL_SDK_DISABLED=true should disable")
	}
	t.Setenv("OTEL_SDK_DISABLED", "")
	if !EnabledFromEnvironment() {
		t.Error("unset should enable")
	}
}

func TestEndpointFromEnvironment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	if got := EndpointFromEnvironment(); got != "collector:4317" {
		t.Errorf("env endpoint = %q", got)
	}
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if got := EndpointFromEnvironment(); got != defaultEndpoint {
		t.Errorf("default endpoint = %q, want %q", got, defaultEndpoint)
	}
}

func TestEndpointURL(t *testing.T) {
	got, err := endpointURL("127.0.0.1:4317")
	if err != nil || got != "http://127.0.0.1:4317" {
		t.Errorf("host:port normalization = %q", got)
	}
	got, err = endpointURL("https://collector:4317")
	if err != nil || got != "https://collector:4317" {
		t.Errorf("complete URL should not be rewritten = %q", got)
	}
	for _, invalid := range []string{
		"collector",
		"collector:",
		":4317",
		"collector:not-a-port",
		"collector:0",
		"collector:70000",
		"://bad",
		"ftp://collector:4317",
		"https://user@collector:4317",
		"https://collector:4317/v1/logs",
	} {
		if _, err := endpointURL(invalid); err == nil {
			t.Errorf("endpointURL(%q) should return error", invalid)
		}
	}
}

func TestNewRuntimeRejectsNegativeDurations(t *testing.T) {
	tests := []Config{
		{Enabled: true, Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputNone}, Trace: TraceConfig{Output: SignalOutputOTLP, BatchTimeout: -1}},
		{Enabled: true, Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputNone}, Metric: MetricConfig{Output: SignalOutputOTLP, ExportInterval: -1}},
		{Enabled: true, Resource: ResourceConfig{ServiceName: "svc"}, Log: LogConfig{Output: SignalOutputOTLP, BatchTimeout: -1}},
	}
	for _, cfg := range tests {
		if _, err := NewRuntime(context.Background(), cfg); err == nil {
			t.Errorf("NewRuntime(%+v) should reject negative duration", cfg)
		}
	}
}

func TestNewRuntimeRejectsInvalidEndpointWhenOTLPRequired(t *testing.T) {
	for _, endpoint := range []string{"collector", ":4317", "collector:not-a-port", "grpc://collector:4317"} {
		_, err := NewRuntime(context.Background(), Config{
			Enabled:  true,
			Resource: ResourceConfig{ServiceName: "svc"},
			Endpoint: endpoint,
			Trace:    TraceConfig{Output: SignalOutputNone},
			Metric:   MetricConfig{Output: SignalOutputNone},
			Log:      LogConfig{Output: SignalOutputOTLP},
		})
		if err == nil {
			t.Errorf("NewRuntime Endpoint=%q error = nil, want non-nil", endpoint)
		}
	}
}

func TestNewRuntimeSkipsEndpointValidationWithoutOTLP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	_, err := NewRuntime(context.Background(), Config{
		Enabled:  true,
		Resource: ResourceConfig{ServiceName: "svc"},
		Endpoint: "collector",
		Trace:    TraceConfig{Output: SignalOutputLocal},
		Metric:   MetricConfig{Output: SignalOutputNone},
		Log:      LogConfig{Output: SignalOutputFile, FilePath: path},
	})
	if err != nil {
		t.Fatalf("non-OTLP runtime should ignore invalid endpoint, got %v", err)
	}
}
