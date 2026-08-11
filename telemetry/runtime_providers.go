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
	counters := new(runtimeCounters)

	traceExporter := cfg.TraceExporter
	ownsTraceExporter := false
	if traceExporter == nil {
		traceExporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
		if err != nil {
			return nil, fmt.Errorf("telemetry: create trace exporter: %w", err)
		}
		ownsTraceExporter = true
	}
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
		loggerProvider = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(countingLogExporter{delegate: logExporter, errors: &counters.logExportErrors},
				sdklog.WithExportInterval(cfg.LogBatchTimeout),
				sdklog.WithExportMaxBatchSize(defaultExportBatchSize),
				sdklog.WithMaxQueueSize(cfg.LogQueueSize),
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
			sdkmetric.WithReader(metricReader),
			sdkmetric.WithResource(res),
		),
		loggerProvider: loggerProvider,
		logOutput:      cfg.LogOutput,
		fileMetadata:   resourceMetadata(res),
		counters:       counters,
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

type countingLogExporter struct {
	delegate sdklog.Exporter
	errors   *atomic.Uint64
}

func (e countingLogExporter) Export(ctx context.Context, records []sdklog.Record) error {
	err := e.delegate.Export(ctx, records)
	if err != nil {
		e.errors.Add(1)
	}
	return err
}

func (e countingLogExporter) Shutdown(ctx context.Context) error {
	return e.delegate.Shutdown(ctx)
}

func (e countingLogExporter) ForceFlush(ctx context.Context) error {
	err := e.delegate.ForceFlush(ctx)
	if err != nil {
		e.errors.Add(1)
	}
	return err
}
