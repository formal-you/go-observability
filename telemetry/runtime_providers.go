package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewRuntime 根据 Config 创建相互独立的 Trace、Metric 和可选 Log Provider。
//
// 构造过程不会修改 OpenTelemetry global state。构造中途失败时，本函数按已取得资源的
// 逆序执行回滚；成功后所有 Provider 及其挂载组件归返回的 Runtime 生命周期管理。
func NewRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
	if !cfg.Enabled {
		if err := normalizeDisabledConfig(&cfg); err != nil {
			return nil, err
		}
		return &Runtime{logConfig: cfg.Log}, nil
	}
	if err := normalizeAndValidateConfig(&cfg); err != nil {
		return nil, err
	}
	var endpoint string
	if otlpEndpointRequired(cfg) {
		var err error
		endpoint, err = endpointURL(defaultIfEmpty(strings.TrimSpace(cfg.Endpoint), defaultEndpoint))
		if err != nil {
			return nil, err
		}
	}
	res := resourceForConfig(cfg.Resource)
	build := newRuntimeBuild()

	switch traceOutput := normalizedTraceOutput(cfg.Trace.Output); traceOutput {
	case SignalOutputNone:
		// 不装配 Trace。
	case SignalOutputLocal:
		build.tracerProvider = newTracerProvider(cfg.Trace, res, nil)
	default:
		traceExporter := cfg.Trace.Exporter
		var traceFile *os.File
		if traceExporter == nil {
			var err error
			traceExporter, traceFile, err = newTraceExporter(ctx, cfg.Trace, traceOutput, endpoint)
			if err != nil {
				build.close(ctx)
				return nil, fmt.Errorf("telemetry: create trace exporter: %w", err)
			}
		}
		build.traceFile = traceFile
		build.tracerProvider = newTracerProvider(cfg.Trace, res, traceExporter)
	}

	switch metricOutput := normalizedMetricOutput(cfg.Metric.Output); metricOutput {
	case SignalOutputNone:
		// 不装配 Metric。
	default:
		metricReader := cfg.Metric.Reader
		var metricFile *os.File
		if metricReader == nil {
			var err error
			metricReader, metricFile, err = newMetricReader(ctx, cfg.Metric, metricOutput, endpoint)
			if err != nil {
				build.close(ctx)
				return nil, fmt.Errorf("telemetry: create metric reader: %w", err)
			}
		}
		build.metricFile = metricFile
		build.meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
			sdkmetric.WithResource(res),
		)
	}

	switch cfg.Log.Output {
	case SignalOutputOTLP:
		logExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpointURL(endpoint))
		if err != nil {
			build.close(ctx)
			return nil, fmt.Errorf("telemetry: create log exporter: %w", err)
		}
		build.loggerProvider = newLoggerProvider(cfg.Log, logExporter, res, build.counters)
	default:
		// file/stdout Writer 由 Runtime.NewWriter 延迟创建；none 不创建。
	}

	return &Runtime{
		resource:       res,
		tracerProvider: build.tracerProvider,
		meterProvider:  build.meterProvider,
		loggerProvider: build.loggerProvider,
		logConfig:      cfg.Log,
		fileMetadata:   resourceMetadata(res),
		counters:       build.counters,
		traceFile:      build.traceFile,
		metricFile:     build.metricFile,
	}, nil
}

// NewFileRuntime 创建不连接 Collector 的 file-only Runtime。
//
// 它是按信号预设的便捷构造器：Trace=local、Metric=none、Log=file。本地 TracerProvider
// 使用 ParentBased(NeverSample())：仍生成合法 TraceID/SpanID 供 JSONL 事件关联，但不
// 记录或导出完整 Span。调用方必须通过 Log.FilePath 提供 JSONL 路径，并在退出时 Shutdown。
func NewFileRuntime(cfg Config) (*Runtime, error) {
	if strings.TrimSpace(cfg.Resource.ServiceName) == "" {
		return nil, errors.New("telemetry: service name is required")
	}
	if cfg.Trace.Output == "" {
		cfg.Trace.Output = SignalOutputLocal
	}
	if cfg.Trace.Output != SignalOutputLocal {
		return nil, fmt.Errorf("telemetry: file-only trace output must be local, got %q", cfg.Trace.Output)
	}
	if cfg.Metric.Output == "" {
		cfg.Metric.Output = SignalOutputNone
	}
	if cfg.Metric.Output != SignalOutputNone {
		return nil, fmt.Errorf("telemetry: file-only metric output must be none, got %q", cfg.Metric.Output)
	}
	if cfg.Log.Output == "" {
		cfg.Log.Output = SignalOutputFile
	}
	if cfg.Log.Output != SignalOutputFile {
		return nil, fmt.Errorf("telemetry: file-only log output must be file, got %q", cfg.Log.Output)
	}
	cfg.Enabled = true
	return NewRuntime(context.Background(), cfg)
}

// NewLogRuntime 创建只装配 Log 的便捷 Runtime：Trace/Metric 不装配，Log 选择 file 或 stdout。
//
// serviceName 是必填服务身份；output 只接受 file 或 stdout。output=file 时 filePath 必填，
// output=stdout 时 filePath 必须为空。构造成功后的 Runtime 与 NewRuntime 相同：不修改
// global state，资源由 Runtime.Shutdown 释放。需要自定义 Log 轮转等参数时，请使用
// NewRuntime 配合 Config.Log。
func NewLogRuntime(ctx context.Context, serviceName string, output SignalOutput, filePath string) (*Runtime, error) {
	if strings.TrimSpace(serviceName) == "" {
		return nil, errors.New("telemetry: service name is required")
	}
	if output != SignalOutputFile && output != SignalOutputStdout {
		return nil, fmt.Errorf("telemetry: log-only output must be file or stdout, got %q", output)
	}
	if output == SignalOutputFile && strings.TrimSpace(filePath) == "" {
		return nil, errors.New("telemetry: log-only file output requires file path")
	}
	if output == SignalOutputStdout && filePath != "" {
		return nil, errors.New("telemetry: log-only stdout output does not accept file path")
	}
	return NewRuntime(ctx, Config{
		Enabled:  true,
		Resource: ResourceConfig{ServiceName: serviceName},
		Trace:    TraceConfig{Output: SignalOutputNone},
		Metric:   MetricConfig{Output: SignalOutputNone},
		Log:      LogConfig{Output: output, FilePath: filePath},
	})
}

// NewOTLPRuntime 创建全 OTLP 的便捷 Runtime：Trace/Metric/Log 三信号都发 Collector。
//
// serviceName 与 endpoint 必填；endpoint 是 OTLP gRPC Collector 地址（k8s 中指向
// Collector Service/DaemonSet）。批量导出间隔、采样比例等使用 telemetry 默认值。
// 需要自定义采样比例、批量间隔或队列容量时，请使用 NewRuntime 配合 Config。
func NewOTLPRuntime(ctx context.Context, serviceName, endpoint string) (*Runtime, error) {
	if strings.TrimSpace(serviceName) == "" {
		return nil, errors.New("telemetry: service name is required")
	}
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("telemetry: endpoint is required")
	}
	return NewRuntime(ctx, Config{
		Enabled:  true,
		Endpoint: endpoint,
		Resource: ResourceConfig{ServiceName: serviceName},
		Trace:    TraceConfig{Output: SignalOutputOTLP},
		Metric:   MetricConfig{Output: SignalOutputOTLP},
		Log:      LogConfig{Output: SignalOutputOTLP},
	})
}

// NewAllFileRuntime 创建全文件输出的便捷 Runtime：Trace/Metric/Log 三信号都写本地文件。
//
// serviceName 与 dir 必填；函数会先创建 dir（如不存在），并在其中生成
// events.jsonl（Log）、trace.jsonl（Trace）、metric.jsonl（Metric）三个文件。
// 与 NewFileRuntime 不同：Trace 使用 file 导出完整 span，而非 local 只生成 TraceID/SpanID。
// 需要自定义批量间隔或轮转时，请使用 NewRuntime 配合 Config 与 LogConfig.FileOptions。
func NewAllFileRuntime(ctx context.Context, serviceName, dir string) (*Runtime, error) {
	if strings.TrimSpace(serviceName) == "" {
		return nil, errors.New("telemetry: service name is required")
	}
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("telemetry: file directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("telemetry: create file directory: %w", err)
	}
	return NewRuntime(ctx, Config{
		Enabled:  true,
		Resource: ResourceConfig{ServiceName: serviceName},
		Trace: TraceConfig{
			Output:   SignalOutputFile,
			FilePath: filepath.Join(dir, "trace.jsonl"),
		},
		Metric: MetricConfig{
			Output:   SignalOutputFile,
			FilePath: filepath.Join(dir, "metric.jsonl"),
		},
		Log: LogConfig{
			Output:   SignalOutputFile,
			FilePath: filepath.Join(dir, "events.jsonl"),
		},
	})
}

// runtimeBuild 集中持有 NewRuntime 已取得的资源，并在构造失败时按逆序回滚。
type runtimeBuild struct {
	counters       *runtimeCounters
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	traceFile      *os.File
	metricFile     *os.File
}

// newRuntimeBuild 创建带诊断计数器的 Runtime 构造回滚容器。
func newRuntimeBuild() *runtimeBuild {
	return &runtimeBuild{counters: new(runtimeCounters)}
}

// close 回滚构造过程中已取得的资源；成功构造后不应再调用。
func (b *runtimeBuild) close(ctx context.Context) {
	if b.loggerProvider != nil {
		_ = b.loggerProvider.Shutdown(ctx)
	}
	if b.meterProvider != nil {
		_ = b.meterProvider.Shutdown(ctx)
	}
	if b.tracerProvider != nil {
		_ = b.tracerProvider.Shutdown(ctx)
	}
	if b.traceFile != nil {
		_ = b.traceFile.Close()
	}
	if b.metricFile != nil {
		_ = b.metricFile.Close()
	}
}

// newTracerProvider 按 TraceConfig 创建 TracerProvider；无 exporter 时使用 NeverSample 本地模式。
func newTracerProvider(cfg TraceConfig, res *resource.Resource, exporter sdktrace.SpanExporter) *sdktrace.TracerProvider {
	if exporter == nil {
		return sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.NeverSample())),
			sdktrace.WithResource(res),
		)
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(cfg.BatchTimeout)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
		sdktrace.WithResource(res),
	)
}

// newLoggerProvider 创建带计数包装、批量导出和 Resource 的 LoggerProvider。
func newLoggerProvider(cfg LogConfig, exporter sdklog.Exporter, res *resource.Resource, counters *runtimeCounters) *sdklog.LoggerProvider {
	// BatchProcessor 提供并发安全的有界队列并在后台批量调用 Exporter。
	// Logger.Emit 只负责把 LogRecord 交给 SDK，不代表 Collector 已经持久化。
	return sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(countingLogExporter{delegate: exporter, errors: &counters.logExportErrors},
			sdklog.WithExportInterval(cfg.BatchTimeout),
			sdklog.WithExportMaxBatchSize(defaultExportBatchSize),
			sdklog.WithMaxQueueSize(cfg.QueueSize),
		)),
		sdklog.WithResource(res),
	)
}

// countingLogExporter 包装 Log Exporter，并把导出失败写入 Runtime 诊断计数。
type countingLogExporter struct {
	delegate sdklog.Exporter
	errors   *atomic.Uint64
}

// Export 把批次原样委托给 OTLP Exporter，并以 atomic 记录可观察的导出失败。
// OTel Log SDK 保证不会并发调用同一 Exporter 的 Export；atomic 仍用于与 Stats 并发读取。
func (e countingLogExporter) Export(ctx context.Context, records []sdklog.Record) error {
	err := e.delegate.Export(ctx, records)
	if err != nil {
		e.errors.Add(1)
	}
	return err
}

// Shutdown 释放底层 Exporter 资源；生命周期并发语义由 OTel Exporter 契约提供。
func (e countingLogExporter) Shutdown(ctx context.Context) error {
	return e.delegate.Shutdown(ctx)
}

// ForceFlush 请求立即导出 SDK 中尚未发送的记录，并统计底层 Exporter 返回的失败。
func (e countingLogExporter) ForceFlush(ctx context.Context) error {
	err := e.delegate.ForceFlush(ctx)
	if err != nil {
		e.errors.Add(1)
	}
	return err
}

// newTraceExporter 按输出目标创建 Trace Exporter：otlp（默认）/ file（stdouttrace +
// WithWriter 写 Trace.FilePath）/ stdout。file 已打开句柄由调用方（Runtime）持有并在
// Shutdown 关闭；本函数失败时自行关闭已打开文件。
func newTraceExporter(ctx context.Context, cfg TraceConfig, output SignalOutput, endpoint string) (sdktrace.SpanExporter, *os.File, error) {
	switch output {
	case SignalOutputFile:
		f, err := openAppendFile(cfg.FilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("telemetry: open trace file: %w", err)
		}
		exporter, err := stdouttrace.New(stdouttrace.WithWriter(f))
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		return exporter, f, nil
	case SignalOutputStdout:
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		return exporter, nil, err
	default: // SignalOutputOTLP
		exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
		return exporter, nil, err
	}
}

// newMetricReader 按输出目标创建 Metric Reader：otlp（默认）/ file（stdoutmetric +
// WithWriter 写 Metric.FilePath）/ stdout。file 已打开句柄由 Runtime 持有并关闭。
func newMetricReader(ctx context.Context, cfg MetricConfig, output SignalOutput, endpoint string) (sdkmetric.Reader, *os.File, error) {
	switch output {
	case SignalOutputFile:
		f, err := openAppendFile(cfg.FilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("telemetry: open metric file: %w", err)
		}
		exporter, err := stdoutmetric.New(stdoutmetric.WithWriter(f))
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.ExportInterval)), f, nil
	case SignalOutputStdout:
		exporter, err := stdoutmetric.New()
		if err != nil {
			return nil, nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.ExportInterval)), nil, nil
	default: // SignalOutputOTLP
		exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(endpoint))
		if err != nil {
			return nil, nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.ExportInterval)), nil, nil
	}
}

// openAppendFile 以追加模式打开输出文件（不存在则创建）。
func openAppendFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
