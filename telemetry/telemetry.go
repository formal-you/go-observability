// Package telemetry 为应用装配 OTel Trace、Metric 和 Log provider。
// Runtime 构造与全局安装相互独立，日志出口由 Config.LogOutput 显式选择。
package telemetry

import (
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/writer/file"
	stdoutwriter "github.com/formal-you/go-observability/writer/stdout"
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
	// LogQueueSize 是 OTLP Log BatchProcessor 的有界队列容量；零值使用 2048。
	LogQueueSize int
	// TraceExporter 允许测试或自定义部署注入公开 OTel SpanExporter。
	// 为空时按 Endpoint 创建 OTLP gRPC exporter。
	TraceExporter sdktrace.SpanExporter
	// MetricReader 允许测试或自定义部署注入公开 OTel Reader。
	// 为空时按 Endpoint 创建 OTLP 周期导出 Reader。
	MetricReader sdkmetric.Reader
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
	counters       *runtimeCounters

	restoreMu   sync.Mutex
	restores    []func()
	shutdown    sync.Once
	shutdownErr error
}

type runtimeCounters struct {
	logExportErrors atomic.Uint64
}

// RuntimeStats 是 Runtime 的导出诊断快照。它不代表后端已持久化，只记录 SDK
// exporter 返回的错误次数；日志队列丢弃仍由 OTel SDK 的内部诊断日志报告。
type RuntimeStats struct {
	LogExportErrors uint64
}

// Stats 返回当前 Runtime 的导出错误快照。
func (r *Runtime) Stats() RuntimeStats {
	if r == nil || r.counters == nil {
		return RuntimeStats{}
	}
	return RuntimeStats{LogExportErrors: r.counters.logExportErrors.Load()}
}

// Providers is retained for source compatibility.
// Deprecated: use Runtime.
type Providers = Runtime

// Tracer returns a tracer from this Runtime, or the process global provider for
// a nil/unconfigured Runtime.
func (r *Runtime) Tracer(name string) trace.Tracer {
	if r == nil || r.tracerProvider == nil {
		return otel.Tracer(name)
	}
	return r.tracerProvider.Tracer(name)
}

// Meter returns a meter from this Runtime, or the process global provider for
// a nil/unconfigured Runtime.
func (r *Runtime) Meter(name string) metric.Meter {
	if r == nil || r.meterProvider == nil {
		return otel.Meter(name)
	}
	return r.meterProvider.Meter(name)
}

// Resource returns the Resource shared by this Runtime's providers and writers.
func (r *Runtime) Resource() *resource.Resource {
	if r == nil {
		return nil
	}
	return r.resource
}

// LoggerProvider returns this Runtime's OTLP LoggerProvider, if configured.
func (r *Runtime) LoggerProvider() *sdklog.LoggerProvider {
	if r == nil {
		return nil
	}
	return r.loggerProvider
}
