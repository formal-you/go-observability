package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
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

	// ownsTraceExporter 只描述构造失败时的回滚责任。成功挂入 TracerProvider 后，
	// 无论 Exporter 是自建还是注入，都会随 Runtime.Shutdown 关闭。
	traceExporter := cfg.TraceExporter
	ownsTraceExporter := false
	if traceExporter == nil {
		traceExporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
		if err != nil {
			return nil, fmt.Errorf("telemetry: create trace exporter: %w", err)
		}
		ownsTraceExporter = true
	}
	// ownsMetricReader 同样只描述构造失败回滚。成功挂入 MeterProvider 后，Reader
	// 随 Runtime.Shutdown 关闭；PeriodicReader 内部负责周期导出。
	metricReader := cfg.MetricReader
	ownsMetricReader := false
	if metricReader == nil {
		metricExporter, metricErr := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(endpoint))
		if metricErr != nil {
			if ownsTraceExporter {
				_ = traceExporter.Shutdown(ctx)
			}
			return nil, fmt.Errorf("telemetry: create metric exporter: %w", metricErr)
		}
		metricReader = sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(cfg.MetricExportInterval))
		ownsMetricReader = true
	}
	var loggerProvider *sdklog.LoggerProvider
	if cfg.LogOutput == LogOutputOTLP {
		logExporter, logErr := otlploggrpc.New(ctx, otlploggrpc.WithEndpointURL(endpoint))
		if logErr != nil {
			if ownsMetricReader {
				_ = metricReader.Shutdown(ctx)
			}
			if ownsTraceExporter {
				_ = traceExporter.Shutdown(ctx)
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
		resource: res,
		tracerProvider: sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(cfg.TraceBatchTimeout)),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
			sdktrace.WithResource(res),
		),
		meterProvider: sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
			sdkmetric.WithResource(res),
		),
		loggerProvider: loggerProvider,
		logOutput:      cfg.LogOutput,
		fileMetadata:   resourceMetadata(res),
		counters:       counters,
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
