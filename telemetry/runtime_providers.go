package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewRuntime 根据 Config 创建相互独立的 Trace、Metric 和可选 Log Provider。
//
// 构造过程不会修改 OpenTelemetry global state。Trace 和 Metric 始终通过 OTLP 或调用方
// 注入的组件导出；只有 LogOutputOTLP 才创建 LoggerProvider。构造中途失败时，本函数按
// 已取得资源的逆序执行回滚，成功后所有 Provider 及其挂载组件归返回的 Runtime 生命周期管理。
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
	counters := new(runtimeCounters)

	// Trace/Metric 按各自 Output 装配：none 时保持 nil（Tracer/Meter 回退进程级 no-op）。
	// file 输出由 newTraceExporter/newMetricReader 打开并由 Runtime 持有（Shutdown 关闭）；
	// 注入的 Exporter/Reader 仍由对应 Provider 随 Shutdown 关闭。
	traceOutput := outputOf(cfg.TraceOutput)
	metricOutput := outputOf(cfg.MetricOutput)

	var traceFile *os.File
	var tracerProvider *sdktrace.TracerProvider
	if traceOutput != LogOutputNone {
		traceExporter := cfg.TraceExporter
		if traceExporter == nil {
			var exporterErr error
			traceExporter, traceFile, exporterErr = newTraceExporter(ctx, cfg, traceOutput, endpoint)
			if exporterErr != nil {
				return nil, fmt.Errorf("telemetry: create trace exporter: %w", exporterErr)
			}
		}
		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(cfg.TraceBatchTimeout)),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
			sdktrace.WithResource(res),
		)
	}

	var metricFile *os.File
	var meterProvider *sdkmetric.MeterProvider
	if metricOutput != LogOutputNone {
		metricReader := cfg.MetricReader
		if metricReader == nil {
			var readerErr error
			metricReader, metricFile, readerErr = newMetricReader(ctx, cfg, metricOutput, endpoint)
			if readerErr != nil {
				if traceFile != nil {
					_ = traceFile.Close()
				}
				return nil, fmt.Errorf("telemetry: create metric reader: %w", readerErr)
			}
		}
		meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
			sdkmetric.WithResource(res),
		)
	}
	var loggerProvider *sdklog.LoggerProvider
	if cfg.LogOutput == LogOutputOTLP {
		logExporter, logErr := otlploggrpc.New(ctx, otlploggrpc.WithEndpointURL(endpoint))
		if logErr != nil {
			if meterProvider != nil {
				_ = meterProvider.Shutdown(ctx)
			}
			if tracerProvider != nil {
				_ = tracerProvider.Shutdown(ctx)
			}
			if traceFile != nil {
				_ = traceFile.Close()
			}
			if metricFile != nil {
				_ = metricFile.Close()
			}
			return nil, fmt.Errorf("telemetry: create log exporter: %w", logErr)
		}
		// BatchProcessor 提供并发安全的有界队列并在后台批量调用 Exporter。
		// Logger.Emit 只负责把 LogRecord 交给 SDK，不代表 Collector 已经持久化。
		loggerProvider = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(countingLogExporter{delegate: logExporter, errors: &counters.logExportErrors},
				sdklog.WithExportInterval(cfg.LogBatchTimeout),
				sdklog.WithExportMaxBatchSize(defaultExportBatchSize),
				sdklog.WithMaxQueueSize(cfg.LogQueueSize),
			)),
			sdklog.WithResource(res),
		)
	}

	// SDK Provider 负责自身公开 API 的并发安全；Runtime 不在 Emit/Record 热路径加锁。
	return &Runtime{
		resource:       res,
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		loggerProvider: loggerProvider,
		logOutput:      cfg.LogOutput,
		fileMetadata:   resourceMetadata(res),
		counters:       counters,
		traceFile:      traceFile,
		metricFile:     metricFile,
	}, nil
}

// NewFileRuntime 创建不连接 Collector 的 file-only Runtime。
//
// 本地 TracerProvider 使用 ParentBased(NeverSample())：仍生成合法 TraceID/SpanID 供
// JSONL 事件关联，但不记录或导出完整 Span。返回的 Runtime 固定使用 LogOutputFile，
// 调用方仍需通过 NewWriter 提供文件路径，并在退出时 Shutdown。
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
// WithWriter 写 TraceFile，参考 example/otel）/ stdout。file 已打开句柄由调用方
// （Runtime）持有并在 Shutdown 关闭；本函数失败时自行关闭已打开文件。
func newTraceExporter(ctx context.Context, cfg Config, output LogOutput, endpoint string) (sdktrace.SpanExporter, *os.File, error) {
	switch output {
	case LogOutputFile:
		f, err := openAppendFile(cfg.TraceFile)
		if err != nil {
			return nil, nil, fmt.Errorf("telemetry: open trace file: %w", err)
		}
		exporter, err := stdouttrace.New(stdouttrace.WithWriter(f))
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		return exporter, f, nil
	case LogOutputStdout:
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		return exporter, nil, err
	default: // LogOutputOTLP
		exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
		return exporter, nil, err
	}
}

// newMetricReader 按输出目标创建 Metric Reader：otlp（默认）/ file（stdoutmetric +
// WithWriter 写 MetricFile）/ stdout。file 已打开句柄由 Runtime 持有并关闭。
func newMetricReader(ctx context.Context, cfg Config, output LogOutput, endpoint string) (sdkmetric.Reader, *os.File, error) {
	switch output {
	case LogOutputFile:
		f, err := openAppendFile(cfg.MetricFile)
		if err != nil {
			return nil, nil, fmt.Errorf("telemetry: open metric file: %w", err)
		}
		exporter, err := stdoutmetric.New(stdoutmetric.WithWriter(f))
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.MetricExportInterval)), f, nil
	case LogOutputStdout:
		exporter, err := stdoutmetric.New()
		if err != nil {
			return nil, nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.MetricExportInterval)), nil, nil
	default: // LogOutputOTLP
		exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(endpoint))
		if err != nil {
			return nil, nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.MetricExportInterval)), nil, nil
	}
}

// openAppendFile 以追加模式打开输出文件（不存在则创建）。
func openAppendFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
