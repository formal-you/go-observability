// Package telemetry 为应用装配 OTel Trace、Metric 和 Log provider。
//
// 本包提供应用可直接导入的三信号装配入口：
//  1. Resource：标准属性（service.name / service.version / deployment.environment）
//     加低基数标签（region / instance）；
//  2. OTLP gRPC exporter：trace / metric / log 各一，默认 127.0.0.1:4317；
//  3. Provider：trace（头部采样 + 批量 5s/512）、metric（PeriodicReader 15s）、
//     log（批量 1s/512），并全局安装（otel / global 包）与 TextMapPropagator；
//  4. Shutdown：退出前 flush 三信号。
//
// 环境变量：SetupFromEnvironment 从 OTEL_SDK_DISABLED 读取启用状态，从
// OTEL_EXPORTER_OTLP_ENDPOINT 读取 endpoint；Setup 时在内存中校验并规范为 URL，
// 不修改 os.Environ。
//
// 采样默认使用 ParentBased(TraceIDRatioBased(0.1))。未被头部采样的 trace 不会到达
// Collector；若要使用 tail_sampling，必须把 TraceSampleRatio 设为 1，让 Collector
// 收到完整 trace 后再做尾部决策。
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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

	"github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/internal/otlpendpoint"
	"github.com/formal-you/go-observability/writer/file"
	"github.com/formal-you/go-observability/writer/otlp"
)

// defaultEndpoint 是未显式配置时的 OTLP gRPC 地址，与 observability/docker-compose.yml
// 中 Collector 暴露的端口一致。
const defaultEndpoint = "127.0.0.1:4317"

// 采集/导出频率缺省值：trace 5s、metric 15s（建议 15-60s）、log 1s。
const (
	defaultTraceBatchTimeout    = 5 * time.Second
	defaultMetricExportInterval = 15 * time.Second
	defaultLogBatchTimeout      = 1 * time.Second
	defaultTraceSampleRatio     = 0.1
	defaultExportBatchSize      = 512
)

// Config 控制本进程的 OTLP pipeline（数据上报管线）。
type Config struct {
	// ServiceName 写入 service.name（Grafana 靠它筛选服务）；非空必填。
	ServiceName string
	// ServiceVersion 写入 service.version，区分部署版本。
	ServiceVersion string
	// Environment 写入 deployment.environment。
	Environment string
	// Region 写入低基数资源属性 region；空则省略。
	Region string
	// Instance 写入资源属性 instance；空则省略。
	Instance string
	// Endpoint OTLP gRPC endpoint（裸 host:port 为明文，URL 保留 http/https）；空用 defaultEndpoint。
	// 显式设置时 NewLogWriter 复用本次 Setup 的 LoggerProvider 写 OTLP。
	Endpoint string
	// Enabled false 时跳过全部初始化，返回空 Providers（进程可离线运行）。
	Enabled bool
	// TraceSampleRatio 头部采样率，默认 0.1；必须落在 (0, 1]。
	TraceSampleRatio float64
	// TraceBatchTimeout trace 批量导出间隔，默认 5s。
	TraceBatchTimeout time.Duration
	// MetricExportInterval metric 周期导出间隔，默认 15s。
	MetricExportInterval time.Duration
	// LogBatchTimeout log 批量导出间隔，默认 1s。
	LogBatchTimeout time.Duration
	// Resource 显式注入的 Resource；nil 时由 ServiceName/Version/Environment/Region/Instance 构建。
	Resource *resource.Resource
}

// Providers 持有本进程的 provider 与共享 Resource。
// 字段未导出，一律经访问器读取：Enabled=false 时 Resource()/LoggerProvider() 为 nil，
// Tracer()/Meter() 回退到全局实现（no-op）。
type Providers struct {
	resource       *resource.Resource
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	useOTLPLogs    bool
}

// Setup 装配并全局安装三信号 provider，返回进程运行时对象。
// 任一步失败返回 error 并关闭已创建的资源，不留半初始化状态。
func Setup(ctx context.Context, cfg Config) (*Providers, error) {
	if !cfg.Enabled {
		return &Providers{}, nil
	}
	if cfg.ServiceName == "" {
		return nil, errors.New("telemetry: service name is required")
	}
	if cfg.TraceSampleRatio == 0 {
		cfg.TraceSampleRatio = defaultTraceSampleRatio
	}
	if cfg.TraceSampleRatio < 0 || cfg.TraceSampleRatio > 1 {
		return nil, errors.New("telemetry: trace sample ratio must be in (0, 1]")
	}
	if cfg.TraceBatchTimeout == 0 {
		cfg.TraceBatchTimeout = defaultTraceBatchTimeout
	}
	if cfg.TraceBatchTimeout < 0 {
		return nil, errors.New("telemetry: trace batch timeout must be positive")
	}
	if cfg.MetricExportInterval == 0 {
		cfg.MetricExportInterval = defaultMetricExportInterval
	}
	if cfg.MetricExportInterval < 0 {
		return nil, errors.New("telemetry: metric export interval must be positive")
	}
	if cfg.LogBatchTimeout == 0 {
		cfg.LogBatchTimeout = defaultLogBatchTimeout
	}
	if cfg.LogBatchTimeout < 0 {
		return nil, errors.New("telemetry: log batch timeout must be positive")
	}
	if cfg.Environment == "" {
		cfg.Environment = "dev"
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	useOTLPLogs := endpoint != ""
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	endpoint, err := endpointURL(endpoint)
	if err != nil {
		return nil, err
	}

	res := cfg.Resource
	if res == nil {
		attrs := []attribute.KeyValue{
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		}
		if cfg.Region != "" {
			attrs = append(attrs, attribute.String("region", cfg.Region))
		}
		if cfg.Instance != "" {
			attrs = append(attrs, attribute.String("instance", cfg.Instance))
		}
		res = resource.NewWithAttributes("https://opentelemetry.io/schemas/1.41.0", attrs...)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
	if err != nil {
		return nil, err
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(endpoint))
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return nil, err
	}
	logExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpointURL(endpoint))
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		_ = metricExporter.Shutdown(ctx)
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(cfg.TraceBatchTimeout)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
		sdktrace.WithResource(res),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(cfg.MetricExportInterval))),
		sdkmetric.WithResource(res),
	)
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter,
			sdklog.WithExportInterval(cfg.LogBatchTimeout),
			sdklog.WithExportMaxBatchSize(defaultExportBatchSize),
		)),
		sdklog.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	global.SetLoggerProvider(loggerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Providers{
		resource:       res,
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		loggerProvider: loggerProvider,
		useOTLPLogs:    useOTLPLogs,
	}, nil
}

// SetupFromEnvironment 按环境变量装配三信号 provider。
// 启用开关从 OTEL_SDK_DISABLED 读取，endpoint 从 OTEL_EXPORTER_OTLP_ENDPOINT 读取
// （缺省 127.0.0.1:4317），其余配置由 cfg 提供；内部调用 Setup。组合根只需调本函数
// + NewLogWriter，不再各自写 env 判断。
func SetupFromEnvironment(ctx context.Context, cfg Config) (*Providers, error) {
	cfg.Enabled = EnabledFromEnvironment()
	if strings.TrimSpace(cfg.Endpoint) == "" {
		cfg.Endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	return Setup(ctx, cfg)
}

// Shutdown 在进程退出前 flush log / metric / trace。
// 顺序刻意：先 log 再 metric 后 trace——log 可能携带 span 上下文，先 flush 保证关联完整。
func (p *Providers) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	var errs []error
	if p.loggerProvider != nil {
		if err := p.loggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Tracer 返回由本进程 runtime 支撑的 Tracer；未启用时回退全局（no-op）。
func (p *Providers) Tracer(name string) trace.Tracer {
	if p == nil || p.tracerProvider == nil {
		return otel.Tracer(name)
	}
	return p.tracerProvider.Tracer(name)
}

// Meter 返回由本进程 runtime 支撑的 Meter；未启用时回退全局（no-op）。
func (p *Providers) Meter(name string) metric.Meter {
	if p == nil || p.meterProvider == nil {
		return otel.Meter(name)
	}
	return p.meterProvider.Meter(name)
}

// Resource 返回共享的 OTel Resource；未启用（Enabled=false）时为 nil，调用方需自行判空。
func (p *Providers) Resource() *resource.Resource {
	if p == nil {
		return nil
	}
	return p.resource
}

// LoggerProvider 返回由本进程 runtime 支撑的日志 LoggerProvider；未启用时为 nil，
// 调用方需自行判空（如 example 用它组装 OTLP Writer 时）。
func (p *Providers) LoggerProvider() *sdklog.LoggerProvider {
	if p == nil {
		return nil
	}
	return p.loggerProvider
}

// NewLogWriter 返回按环境选择的日志 Writer。
// Setup 时显式配置 endpoint（含 SetupFromEnvironment 读取到环境变量）且 Providers
// 已启用 → OTLP Writer（复用本进程的 Resource 与 LoggerProvider）；否则写本地 JSONL。
// 未启用（空 Providers）时走 file Writer，保证本地离线可核对。
func (p *Providers) NewLogWriter(ctx context.Context, jsonlPath string) (log.Writer, error) {
	if p != nil && p.useOTLPLogs && p.LoggerProvider() != nil {
		opts := []otlp.Option{otlp.WithLoggerProvider(p.LoggerProvider())}
		if res := p.Resource(); res != nil {
			opts = append(opts, otlp.WithResource(res))
		}
		return otlp.New(ctx, opts...)
	}
	return file.New(jsonlPath)
}

// EnabledFromEnvironment 从 OTEL_SDK_DISABLED 读取启用开关。
func EnabledFromEnvironment() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true")
}

// EndpointFromEnvironment 返回 OTLP gRPC endpoint：OTEL_EXPORTER_OTLP_ENDPOINT，缺省 127.0.0.1:4317。
func EndpointFromEnvironment() string {
	if endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return defaultEndpoint
}

// endpointURL 校验并规范化 endpoint：接受 host:port 与 http(s) URL 两种形式。
func endpointURL(endpoint string) (string, error) {
	normalized, err := otlpendpoint.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("telemetry: %w", err)
	}
	return normalized, nil
}
