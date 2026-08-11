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

	"github.com/formal-you/go-observability/writer/file"
	stdoutwriter "github.com/formal-you/go-observability/writer/stdout"
)

func TestNewRuntimeDoesNotInstallGlobals(t *testing.T) {
	oldTrace := otel.GetTracerProvider()
	oldMeter := otel.GetMeterProvider()
	oldLogger := global.GetLoggerProvider()
	oldPropagator := otel.GetTextMapPropagator()
	r, err := NewRuntime(context.Background(), Config{
		Enabled: true, ServiceName: "svc", LogOutput: LogOutputNone,
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
	customResource := resourceForConfig(Config{ServiceName: "resource-only"})
	tests := []Config{
		{Enabled: true, LogOutput: LogOutputFile},
		{Enabled: true, LogOutput: LogOutputFile, Resource: customResource},
		{Enabled: true, ServiceName: "svc"},
		{Enabled: true, ServiceName: "svc", LogOutput: "invalid"},
	}
	for _, cfg := range tests {
		if _, err := NewRuntime(context.Background(), cfg); err == nil {
			t.Errorf("NewRuntime(%+v) error = nil", cfg)
		}
	}
	r, err := NewRuntime(context.Background(), Config{})
	if err != nil || r == nil || r.Resource() != nil {
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

	r1, err := NewRuntime(context.Background(), Config{Enabled: true, ServiceName: "one", LogOutput: LogOutputNone, TraceBatchTimeout: time.Hour, MetricExportInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := NewRuntime(context.Background(), Config{Enabled: true, ServiceName: "two", LogOutput: LogOutputNone, TraceBatchTimeout: time.Hour, MetricExportInterval: time.Hour})
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
	r, err := NewFileRuntime(Config{ServiceName: "mall", ServiceVersion: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(context.Background())
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := r.NewWriter(context.Background(), WriterConfig{FilePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "business", slog.String("event.name", "order.payment.succeeded")); err != nil {
		t.Fatal(err)
	}
	if closer, ok := w.(interface{ Close(context.Context) error }); ok {
		if err := closer.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
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
	r := &Runtime{resource: resourceForConfig(Config{ServiceName: "stdout-svc", Environment: "test"}), logOutput: LogOutputStdout}
	var buf bytes.Buffer
	w, err := r.NewWriter(context.Background(), WriterConfig{StdoutOptions: []stdoutwriter.Option{stdoutwriter.WithOutput(&buf)}})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "access"); err != nil {
		t.Fatal(err)
	}
	if closer, ok := w.(interface{ Close(context.Context) error }); ok {
		if err := closer.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if out := buf.String(); !strings.Contains(out, "stdout-svc") || !strings.Contains(out, "access") {
		t.Fatalf("stdout output missing resource or event: %s", out)
	}
}

func TestNewWriterRejectsMismatchedConfiguration(t *testing.T) {
	dummyFileOption := file.WithResourceMetadata(file.ResourceMetadata{})
	tests := []struct {
		runtime *Runtime
		cfg     WriterConfig
	}{
		{&Runtime{logOutput: LogOutputFile}, WriterConfig{}},
		{&Runtime{logOutput: LogOutputFile}, WriterConfig{FilePath: "x", StdoutOptions: []stdoutwriter.Option{stdoutwriter.WithOutput(&bytes.Buffer{})}}},
		{&Runtime{logOutput: LogOutputOTLP, loggerProvider: sdklog.NewLoggerProvider()}, WriterConfig{FilePath: "x"}},
		{&Runtime{logOutput: LogOutputStdout}, WriterConfig{FileOptions: []file.Option{dummyFileOption}}},
		{&Runtime{logOutput: LogOutputNone}, WriterConfig{FilePath: "x"}},
	}
	for _, tt := range tests {
		if _, err := tt.runtime.NewWriter(context.Background(), tt.cfg); err == nil {
			t.Errorf("NewWriter(%q, %+v) error = nil", tt.runtime.logOutput, tt.cfg)
		}
	}
}

func TestNewWriterNoneIsNoop(t *testing.T) {
	w, err := (&Runtime{logOutput: LogOutputNone}).NewWriter(context.Background(), WriterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "ignored", slog.String("key", "value")); err != nil {
		t.Fatal(err)
	}
}

func TestSetupDisabled(t *testing.T) {
	ctx := context.Background()
	p, err := Setup(ctx, Config{Enabled: false})
	if err != nil {
		t.Fatalf("disabled setup 不应报错: %v", err)
	}
	if p == nil || p.Resource() != nil || p.LoggerProvider() != nil {
		t.Fatalf("disabled setup 应返回空 Providers，got %+v", p)
	}
	if p.Tracer("x") == nil {
		t.Error("disabled setup 的 Tracer 应回退全局（非 nil）")
	}
	if p.Meter("x") == nil {
		t.Error("disabled setup 的 Meter 应回退全局（非 nil）")
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("disabled setup 的 Shutdown 应返回 nil, got %v", err)
	}
}

func TestSetupDisabledRetainsLegacyFileWriter(t *testing.T) {
	ctx := context.Background()
	r, err := Setup(ctx, Config{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "disabled.jsonl")
	w, err := r.NewLogWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if closer, ok := w.(interface{ Close(context.Context) error }); ok {
		if err := closer.Close(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSetupRequiresServiceName(t *testing.T) {
	_, err := Setup(context.Background(), Config{Enabled: true})
	if err == nil {
		t.Fatal("缺少 service.name 应报错")
	}
}

func TestSetupInvalidSampleRatio(t *testing.T) {
	for _, ratio := range []float64{-0.1, 1.5} {
		if _, err := Setup(context.Background(), Config{Enabled: true, ServiceName: "svc", TraceSampleRatio: ratio}); err == nil {
			t.Errorf("ratio=%v 应报错", ratio)
		}
	}
}

func TestSetupResourceAttributes(t *testing.T) {
	ctx := context.Background()
	p, err := Setup(ctx, Config{
		Enabled:        true,
		ServiceName:    "svc",
		ServiceVersion: "1.2.3",
		Environment:    "test",
		Region:         "cn",
		Instance:       "i-1",
		Endpoint:       "127.0.0.1:4317",
	})
	if err != nil {
		t.Fatalf("setup 失败: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()
	if p.Resource() == nil || p.LoggerProvider() == nil {
		t.Fatal("enabled setup 应返回非 nil 的 Resource 与 LoggerProvider")
	}

	want := map[string]string{
		"service.name":                "svc",
		"service.version":             "1.2.3",
		"deployment.environment.name": "test",
		"region":                      "cn",
		"service.instance.id":         "i-1",
	}
	got := map[string]string{}
	for _, kv := range p.Resource().Attributes() {
		got[string(kv.Key)] = kv.Value.Emit()
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("resource 属性 %s = %q, want %q（全部: %v）", k, got[k], v, got)
		}
	}
}

func TestSetupFileCreatesLocalTracesAndMetadata(t *testing.T) {
	ctx := context.Background()
	p, err := SetupFile(Config{ServiceName: "mall-monolith", ServiceVersion: "1.0.0", Instance: "shop-01"})
	if err != nil {
		t.Fatalf("SetupFile 失败: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()
	if p.LoggerProvider() != nil {
		t.Fatal("SetupFile 不应创建 OTLP LoggerProvider")
	}
	attrs := map[string]string{}
	for _, kv := range p.Resource().Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	if attrs["service.name"] != "mall-monolith" || attrs["service.version"] != "1.0.0" || attrs["service.instance.id"] != "shop-01" || attrs["deployment.environment.name"] != "development" {
		t.Fatalf("SetupFile Resource = %v", attrs)
	}
	tracer := p.Tracer("test")
	_, span1 := tracer.Start(ctx, "one")
	_, span2 := tracer.Start(ctx, "two")
	if !span1.SpanContext().TraceID().IsValid() || !span1.SpanContext().SpanID().IsValid() {
		t.Fatal("本地 span1 ID 无效")
	}
	if span1.SpanContext().IsSampled() {
		t.Fatal("file-only root span 不应设置 sampled 标记")
	}
	if !span2.SpanContext().TraceID().IsValid() || span1.SpanContext().TraceID() == span2.SpanContext().TraceID() {
		t.Fatal("独立本地 root span 应有不同有效 TraceID")
	}
	span1.End()
	span2.End()
}

func TestSetupFileRequiresServiceName(t *testing.T) {
	if _, err := SetupFile(Config{}); err == nil {
		t.Fatal("SetupFile 缺少 service.name 应报错")
	}
}

func TestShutdownNilAndDisabled(t *testing.T) {
	ctx := context.Background()
	if err := (*Providers)(nil).Shutdown(ctx); err != nil {
		t.Errorf("nil Shutdown 应返回 nil, got %v", err)
	}
	if err := (&Providers{}).Shutdown(ctx); err != nil {
		t.Errorf("空 Providers Shutdown 应返回 nil, got %v", err)
	}
}

func TestEnabledFromEnvironment(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	if EnabledFromEnvironment() {
		t.Error("OTEL_SDK_DISABLED=true 时应禁用")
	}
	t.Setenv("OTEL_SDK_DISABLED", "")
	if !EnabledFromEnvironment() {
		t.Error("未设置时应默认启用")
	}
}

func TestEndpointFromEnvironment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	if got := EndpointFromEnvironment(); got != "collector:4317" {
		t.Errorf("env endpoint = %q", got)
	}
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if got := EndpointFromEnvironment(); got != defaultEndpoint {
		t.Errorf("缺省 endpoint = %q, want %q", got, defaultEndpoint)
	}
}

func TestEndpointURL(t *testing.T) {
	got, err := endpointURL("127.0.0.1:4317")
	if err != nil || got != "http://127.0.0.1:4317" {
		t.Errorf("host:port 规范化 = %q", got)
	}
	got, err = endpointURL("https://collector:4317")
	if err != nil || got != "https://collector:4317" {
		t.Errorf("完整 URL 不应改写 = %q", got)
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
			t.Errorf("endpointURL(%q) 应返回错误", invalid)
		}
	}
}

func TestSetupRejectsNegativeDurations(t *testing.T) {
	tests := []Config{
		{Enabled: true, ServiceName: "svc", TraceBatchTimeout: -1},
		{Enabled: true, ServiceName: "svc", MetricExportInterval: -1},
		{Enabled: true, ServiceName: "svc", LogBatchTimeout: -1},
	}
	for _, cfg := range tests {
		if _, err := Setup(context.Background(), cfg); err == nil {
			t.Errorf("Setup(%+v) 应拒绝负数周期", cfg)
		}
	}
}

func TestSetupRejectsInvalidEndpoint(t *testing.T) {
	for _, endpoint := range []string{"collector", ":4317", "collector:not-a-port", "grpc://collector:4317"} {
		_, err := Setup(context.Background(), Config{
			Enabled:     true,
			ServiceName: "svc",
			Endpoint:    endpoint,
		})
		if err == nil {
			t.Errorf("Setup Endpoint=%q error = nil, want non-nil", endpoint)
		}
	}
}
