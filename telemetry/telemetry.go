// Package telemetry 为应用装配 OTel Trace、Metric 和 Log provider。
// Runtime 构造与全局安装相互独立，日志出口由 Config.LogOutput 显式选择。
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/internal/otlpendpoint"
	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/writer/file"
	"github.com/formal-you/go-observability/writer/otlp"
	stdoutwriter "github.com/formal-you/go-observability/writer/stdout"
)

const defaultEndpoint = "127.0.0.1:4317"

const (
	defaultTraceBatchTimeout    = 5 * time.Second
	defaultMetricExportInterval = 15 * time.Second
	defaultLogBatchTimeout      = 1 * time.Second
	defaultTraceSampleRatio     = 0.1
	defaultExportBatchSize      = 512
)

// LogOutput selects the log Writer created by Runtime.NewWriter.
type LogOutput string

const (
	LogOutputFile   LogOutput = "file"
	LogOutputOTLP   LogOutput = "otlp"
	LogOutputStdout LogOutput = "stdout"
	LogOutputNone   LogOutput = "none"
)

// Config controls this process's telemetry runtime.
type Config struct {
	// ServiceName 写入 service.name；Enabled=true 时必填。
	ServiceName string
	// ServiceVersion 写入 service.version。
	ServiceVersion string
	// Environment 写入 deployment.environment.name；为空时默认 development。
	Environment string
	// Region 写入低基数资源属性 region。
	Region string
	// Instance 写入 service.instance.id。
	Instance string
	// Endpoint 只配置 OTLP gRPC 地址；为空时使用 127.0.0.1:4317。
	Endpoint string
	// Enabled=false 返回不含 provider 的空 Runtime。
	Enabled bool
	// LogOutput 显式选择 file、otlp、stdout 或 none。
	LogOutput LogOutput
	// TraceSampleRatio 是 SDK 头部采样率；零值使用 0.1。
	TraceSampleRatio float64
	// TraceBatchTimeout 是 span 批量导出间隔；零值使用 5s。
	TraceBatchTimeout time.Duration
	// MetricExportInterval 是 metric 周期导出间隔；零值使用 15s。
	MetricExportInterval time.Duration
	// LogBatchTimeout 是 log 批量导出间隔；零值使用 1s。
	LogBatchTimeout time.Duration
	// Resource 可覆盖由服务身份字段构建的 Resource。
	Resource *resource.Resource
}

// WriterConfig contains options for the selected Runtime log output.
// Options for another output are rejected instead of silently ignored.
type WriterConfig struct {
	FilePath      string
	FileOptions   []file.Option
	StdoutOptions []stdoutwriter.Option
}

// Runtime owns telemetry providers and their shared Resource. Constructing a
// Runtime does not change OpenTelemetry global state.
type Runtime struct {
	resource       *resource.Resource
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	logOutput      LogOutput
	fileMetadata   file.ResourceMetadata

	restoreMu   sync.Mutex
	restores    []func()
	shutdown    sync.Once
	shutdownErr error
}

// Providers is retained for source compatibility.
// Deprecated: use Runtime.
type Providers = Runtime

// NewRuntime creates OTLP-backed providers without installing them globally.
func NewRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
	if !cfg.Enabled {
		output := cfg.LogOutput
		if output == "" || output == LogOutputOTLP {
			output = LogOutputFile
		}
		return &Runtime{logOutput: output}, nil
	}
	if err := normalizeAndValidateConfig(&cfg); err != nil {
		return nil, err
	}
	endpoint, err := endpointURL(defaultIfEmpty(strings.TrimSpace(cfg.Endpoint), defaultEndpoint))
	if err != nil {
		return nil, err
	}
	res := resourceForConfig(cfg)

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
	if err != nil {
		return nil, fmt.Errorf("telemetry: create trace exporter: %w", err)
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(endpoint))
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return nil, fmt.Errorf("telemetry: create metric exporter: %w", err)
	}
	var loggerProvider *sdklog.LoggerProvider
	if cfg.LogOutput == LogOutputOTLP {
		logExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpointURL(endpoint))
		if err != nil {
			_ = metricExporter.Shutdown(ctx)
			_ = traceExporter.Shutdown(ctx)
			return nil, fmt.Errorf("telemetry: create log exporter: %w", err)
		}
		loggerProvider = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter,
				sdklog.WithExportInterval(cfg.LogBatchTimeout),
				sdklog.WithExportMaxBatchSize(defaultExportBatchSize),
			)),
			sdklog.WithResource(res),
		)
	}

	return &Runtime{
		resource: res,
		tracerProvider: sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(cfg.TraceBatchTimeout)),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
			sdktrace.WithResource(res),
		),
		meterProvider: sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(cfg.MetricExportInterval))),
			sdkmetric.WithResource(res),
		),
		loggerProvider: loggerProvider,
		logOutput:      cfg.LogOutput,
		fileMetadata:   resourceMetadata(res),
	}, nil
}

// NewFileRuntime creates a local NeverSample trace provider and no exporter.
func NewFileRuntime(cfg Config) (*Runtime, error) {
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return nil, errors.New("telemetry: service name is required")
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	res := resourceForConfig(cfg)
	return &Runtime{
		resource: res,
		tracerProvider: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.NeverSample())),
			sdktrace.WithResource(res),
		),
		logOutput:    LogOutputFile,
		fileMetadata: resourceMetadata(res),
	}, nil
}

func normalizeAndValidateConfig(cfg *Config) error {
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return errors.New("telemetry: service name is required")
	}
	switch cfg.LogOutput {
	case LogOutputFile, LogOutputOTLP, LogOutputStdout, LogOutputNone:
	default:
		return fmt.Errorf("telemetry: invalid log output %q", cfg.LogOutput)
	}
	if cfg.TraceSampleRatio == 0 {
		cfg.TraceSampleRatio = defaultTraceSampleRatio
	}
	if cfg.TraceSampleRatio < 0 || cfg.TraceSampleRatio > 1 {
		return errors.New("telemetry: trace sample ratio must be in (0, 1]")
	}
	if cfg.TraceBatchTimeout == 0 {
		cfg.TraceBatchTimeout = defaultTraceBatchTimeout
	}
	if cfg.TraceBatchTimeout < 0 {
		return errors.New("telemetry: trace batch timeout must be positive")
	}
	if cfg.MetricExportInterval == 0 {
		cfg.MetricExportInterval = defaultMetricExportInterval
	}
	if cfg.MetricExportInterval < 0 {
		return errors.New("telemetry: metric export interval must be positive")
	}
	if cfg.LogBatchTimeout == 0 {
		cfg.LogBatchTimeout = defaultLogBatchTimeout
	}
	if cfg.LogBatchTimeout < 0 {
		return errors.New("telemetry: log batch timeout must be positive")
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	return nil
}

// InstallGlobal installs this Runtime's non-nil providers and the W3C
// propagator. The returned restore function is idempotent. Nested installs are
// supported when their restore functions are called in LIFO order.
func (r *Runtime) InstallGlobal() func() {
	if r == nil || (r.tracerProvider == nil && r.meterProvider == nil && r.loggerProvider == nil) {
		return func() {}
	}
	oldTrace := otel.GetTracerProvider()
	oldMeter := otel.GetMeterProvider()
	oldLogger := global.GetLoggerProvider()
	oldPropagator := otel.GetTextMapPropagator()
	if r.tracerProvider != nil {
		otel.SetTracerProvider(r.tracerProvider)
	}
	if r.meterProvider != nil {
		otel.SetMeterProvider(r.meterProvider)
	}
	if r.loggerProvider != nil {
		global.SetLoggerProvider(r.loggerProvider)
	}
	otel.SetTextMapPropagator(w3cPropagator())

	var once sync.Once
	restore := func() {
		once.Do(func() {
			if r.loggerProvider != nil {
				global.SetLoggerProvider(oldLogger)
			}
			if r.meterProvider != nil {
				otel.SetMeterProvider(oldMeter)
			}
			if r.tracerProvider != nil {
				otel.SetTracerProvider(oldTrace)
			}
			otel.SetTextMapPropagator(oldPropagator)
		})
	}
	r.restoreMu.Lock()
	r.restores = append(r.restores, restore)
	r.restoreMu.Unlock()
	return restore
}

func w3cPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

// NewWriter creates the explicitly selected log output.
func (r *Runtime) NewWriter(ctx context.Context, cfg WriterConfig) (log.Writer, error) {
	if r == nil {
		return nil, errors.New("telemetry: nil runtime")
	}
	switch r.logOutput {
	case LogOutputFile:
		if strings.TrimSpace(cfg.FilePath) == "" {
			return nil, errors.New("telemetry: file output requires file path")
		}
		if len(cfg.StdoutOptions) != 0 {
			return nil, errors.New("telemetry: stdout options do not apply to file output")
		}
		opts := append([]file.Option(nil), cfg.FileOptions...)
		opts = append(opts, file.WithResourceMetadata(r.fileMetadata))
		return file.New(cfg.FilePath, opts...)
	case LogOutputOTLP:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 || len(cfg.StdoutOptions) != 0 {
			return nil, errors.New("telemetry: writer options do not apply to otlp output")
		}
		if r.loggerProvider == nil {
			return nil, errors.New("telemetry: otlp output requires logger provider")
		}
		return otlp.New(ctx, otlp.WithLoggerProvider(r.loggerProvider), otlp.WithResource(r.resource))
	case LogOutputStdout:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 {
			return nil, errors.New("telemetry: file options do not apply to stdout output")
		}
		opts := append([]stdoutwriter.Option(nil), cfg.StdoutOptions...)
		opts = append(opts, stdoutwriter.WithResource(r.resource))
		return stdoutwriter.New(ctx, opts...)
	case LogOutputNone:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 || len(cfg.StdoutOptions) != 0 {
			return nil, errors.New("telemetry: writer options do not apply to none output")
		}
		return noopWriter{}, nil
	default:
		return nil, fmt.Errorf("telemetry: invalid log output %q", r.logOutput)
	}
}

type noopWriter struct{}

func (noopWriter) Write(context.Context, string, ...slog.Attr) error { return nil }

// Setup creates a Runtime using legacy endpoint-based output selection and
// installs it globally.
// Deprecated: use NewRuntime and Runtime.InstallGlobal.
func Setup(ctx context.Context, cfg Config) (*Runtime, error) {
	if cfg.Enabled {
		if strings.TrimSpace(cfg.Endpoint) != "" {
			cfg.LogOutput = LogOutputOTLP
		} else {
			cfg.LogOutput = LogOutputFile
		}
	}
	r, err := NewRuntime(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		r.logOutput = LogOutputFile
	}
	r.InstallGlobal()
	return r, nil
}

// SetupFile creates a local file Runtime and installs it globally.
// Deprecated: use NewFileRuntime and Runtime.InstallGlobal.
func SetupFile(cfg Config) (*Runtime, error) {
	r, err := NewFileRuntime(cfg)
	if err != nil {
		return nil, err
	}
	r.InstallGlobal()
	return r, nil
}

// SetupFromEnvironment applies the legacy environment mapping before Setup.
// Deprecated: use NewRuntime and explicit Config fields.
func SetupFromEnvironment(ctx context.Context, cfg Config) (*Runtime, error) {
	cfg.Enabled = EnabledFromEnvironment()
	if strings.TrimSpace(cfg.Endpoint) == "" {
		cfg.Endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	return Setup(ctx, cfg)
}

// NewLogWriter maps the legacy path and file options to NewWriter.
// Deprecated: use Runtime.NewWriter.
func (r *Runtime) NewLogWriter(ctx context.Context, jsonlPath string, fileOpts ...file.Option) (log.Writer, error) {
	if r != nil && r.logOutput == LogOutputOTLP {
		return r.NewWriter(ctx, WriterConfig{})
	}
	return r.NewWriter(ctx, WriterConfig{FilePath: jsonlPath, FileOptions: fileOpts})
}

// Shutdown restores globals first, then shuts down log, metric, and trace.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.shutdown.Do(func() {
		r.restoreMu.Lock()
		for i := len(r.restores) - 1; i >= 0; i-- {
			r.restores[i]()
		}
		r.restoreMu.Unlock()
		var errs []error
		if r.loggerProvider != nil {
			errs = appendError(errs, r.loggerProvider.Shutdown(ctx))
		}
		if r.meterProvider != nil {
			errs = appendError(errs, r.meterProvider.Shutdown(ctx))
		}
		if r.tracerProvider != nil {
			errs = appendError(errs, r.tracerProvider.Shutdown(ctx))
		}
		r.shutdownErr = errors.Join(errs...)
	})
	return r.shutdownErr
}

func appendError(errs []error, err error) []error {
	if err != nil {
		return append(errs, err)
	}
	return errs
}

func (r *Runtime) Tracer(name string) trace.Tracer {
	if r == nil || r.tracerProvider == nil {
		return otel.Tracer(name)
	}
	return r.tracerProvider.Tracer(name)
}

func (r *Runtime) Meter(name string) metric.Meter {
	if r == nil || r.meterProvider == nil {
		return otel.Meter(name)
	}
	return r.meterProvider.Meter(name)
}

func (r *Runtime) Resource() *resource.Resource {
	if r == nil {
		return nil
	}
	return r.resource
}

func (r *Runtime) LoggerProvider() *sdklog.LoggerProvider {
	if r == nil {
		return nil
	}
	return r.loggerProvider
}

func resourceForConfig(cfg Config) *resource.Resource {
	if cfg.Resource != nil {
		return cfg.Resource
	}
	attrs := make([]attribute.KeyValue, 0, 5)
	if cfg.ServiceName != "" {
		attrs = append(attrs, attribute.String("service.name", cfg.ServiceName))
	}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, attribute.String("service.version", cfg.ServiceVersion))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment.name", cfg.Environment))
	}
	if cfg.Region != "" {
		attrs = append(attrs, attribute.String("region", cfg.Region))
	}
	if cfg.Instance != "" {
		attrs = append(attrs, attribute.String("service.instance.id", cfg.Instance))
	}
	return resource.NewWithAttributes("https://opentelemetry.io/schemas/1.41.0", attrs...)
}

func resourceMetadata(res *resource.Resource) file.ResourceMetadata {
	var metadata file.ResourceMetadata
	if res == nil {
		return metadata
	}
	for _, kv := range res.Attributes() {
		switch string(kv.Key) {
		case "service.name":
			metadata.ServiceName = kv.Value.AsString()
		case "service.version":
			metadata.ServiceVersion = kv.Value.AsString()
		case "service.instance.id":
			metadata.ServiceInstanceID = kv.Value.AsString()
		case "deployment.environment.name":
			metadata.DeploymentEnvironmentName = kv.Value.AsString()
		}
	}
	return metadata
}

func EnabledFromEnvironment() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true")
}

func EndpointFromEnvironment() string {
	return defaultIfEmpty(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")), defaultEndpoint)
}

func endpointURL(endpoint string) (string, error) {
	normalized, err := otlpendpoint.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("telemetry: %w", err)
	}
	return normalized, nil
}

func defaultIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
