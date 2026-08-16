// Package telemetry 为应用装配 OpenTelemetry Trace、Metric 和 Log Provider。
//
// Runtime 是本包的资源所有权边界：它持有由 Config 创建的 Provider，并负责按
// Log -> Metric -> Trace 的顺序关闭。NewRuntime 只构造独立 Runtime，不修改进程级
// OpenTelemetry global state；需要让依赖全局 API 的组件使用这些 Provider 时，调用
// Runtime.InstallGlobal，并在退出时调用其 restore 函数。
//
// 每个信号通过独立 Config 分组显式选择出口。OTLP 出口把并发入队、批量导出和 gRPC
// 连接交给 OTel SDK；file 出口则由文件 Writer 自行同步并发写入和轮转。
package telemetry

import (
	"os"
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

	"github.com/formal-you/go-observability/logwriter/file"
	stdoutwriter "github.com/formal-you/go-observability/logwriter/stdout"
)

// SignalOutput 表示某个 OpenTelemetry 信号的出口类型。
type SignalOutput string

const (
	// SignalOutputFile 把信号写为本地文件。
	SignalOutputFile SignalOutput = "file"
	// SignalOutputOTLP 通过 OTel SDK 的 OTLP/gRPC Exporter 把信号发送到 Collector。
	SignalOutputOTLP SignalOutput = "otlp"
	// SignalOutputStdout 使用 OTel stdout Exporter 输出信号，适合本地开发和诊断。
	SignalOutputStdout SignalOutput = "stdout"
	// SignalOutputNone 不装配对应 Provider；获取 Tracer/Meter 时回退进程级 no-op。
	SignalOutputNone SignalOutput = "none"
	// SignalOutputLocal 仅用于 Trace：创建只生成合法 TraceID/SpanID 但不导出完整
	// Span 的本地 TracerProvider，供 file-only 日志链路关联使用。
	SignalOutputLocal SignalOutput = "local"
)

// ResourceConfig 描述写入 OTel Resource 的进程级服务身份。
//
// Override 不为 nil 时直接使用调用方提供的 Resource，调用方负责其属性完整性；
// 其余字段按 OTel semconv 1.41.0 构建低基数服务身份。
type ResourceConfig struct {
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
	// Override 覆盖由服务身份字段构建的 OTel Resource；注入对象由调用方拥有。
	Override *resource.Resource
}

// TraceConfig 定义 Trace 信号的装配参数。
type TraceConfig struct {
	// Output 选择 otlp（默认）/ file / stdout / none / local。
	Output SignalOutput
	// FilePath 是 Output=file 时的写入路径，其他输出必须留空。
	FilePath string
	// SampleRatio 是 SDK head sampling 比例；零值使用 0.1。
	// 未被采样的 Trace 不会发送到 Collector，Collector tail sampling 无法恢复。
	SampleRatio float64
	// BatchTimeout 是 SpanProcessor 的 SDK 批量导出间隔；零值使用 5s。
	BatchTimeout time.Duration
	// Exporter 允许测试或自定义部署注入公开 OTel SpanExporter，仅 Output=otlp 时可使用。
	Exporter sdktrace.SpanExporter
}

// MetricConfig 定义 Metric 信号的装配参数。
type MetricConfig struct {
	// Output 选择 otlp（默认）/ file / stdout / none。
	Output SignalOutput
	// FilePath 是 Output=file 时的写入路径，其他输出必须留空。
	FilePath string
	// ExportInterval 是 Metric Reader 的 SDK 周期导出间隔；零值使用 15s。
	ExportInterval time.Duration
	// Reader 允许测试或自定义部署注入公开 OTel Reader，仅 Output=otlp 时可使用。
	Reader sdkmetric.Reader
}

// LogConfig 定义 Log 信号与 Runtime.NewWriter 的装配参数。
type LogConfig struct {
	// Output 显式选择 file、otlp、stdout 或 none。
	Output SignalOutput
	// FilePath 是 Output=file 的 JSONL 路径，其他输出必须留空。
	FilePath string
	// FileOptions 只用于 Output=file；Runtime 会在其后注入自身 Resource metadata。
	FileOptions []file.Option
	// StdoutOptions 只用于 Output=stdout。
	StdoutOptions []stdoutwriter.Option
	// BatchTimeout 是 Log BatchProcessor 的 SDK 批量导出间隔；零值使用 1s。
	BatchTimeout time.Duration
	// QueueSize 是 OTLP Log BatchProcessor 的有界队列容量；零值使用 2048。
	// 队列满时的丢弃由 OTel SDK 诊断机制报告，不由 Logger.Emit 返回给调用方。
	QueueSize int
}

// Config 定义当前进程 Telemetry Runtime 的装配参数。
//
// Config 只在构造 Runtime 时读取；构造完成后修改 Config 不会动态更新 Provider。
// Enabled=true 时 Resource.ServiceName 和 Log.Output 必填。时间字段表示 SDK 侧的
// 批量导出周期，不是 sampling，也不是 Collector 的 batch processor 配置。
type Config struct {
	// Enabled 控制是否装配 Telemetry Provider；false 返回不含 Provider 的空 Runtime。
	Enabled bool
	// Endpoint 配置 OTLP/gRPC Collector 地址；为空时使用 127.0.0.1:4317。
	// 它不用于推断任何信号的 Output；只在确有信号选择 OTLP 时被解析。
	Endpoint string
	// Resource 是服务身份与 Resource 覆盖。
	Resource ResourceConfig
	// Trace 是 Trace 信号配置。
	Trace TraceConfig
	// Metric 是 Metric 信号配置。
	Metric MetricConfig
	// Log 是 Log 信号配置。
	Log LogConfig
}

// Runtime 持有 Telemetry Provider 及其共享的 Resource。
//
// Runtime 构造完成后，Provider、Resource 和出口配置保持只读，可由多个 goroutine
// 获取 Tracer、Meter 或 Writer。restoreMu 只保护 InstallGlobal 登记的恢复栈；shutdown
// 使用 sync.Once 保证 Provider 只关闭一次；OTLP 热路径不经过这两个锁。
// 构造 Runtime 本身不会修改 OpenTelemetry global state。
type Runtime struct {
	resource       *resource.Resource
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	logConfig      LogConfig
	fileMetadata   file.ResourceMetadata
	counters       *runtimeCounters
	traceFile      *os.File // Trace.Output=file 时持有，Shutdown 关闭
	metricFile     *os.File // Metric.Output=file 时持有，Shutdown 关闭

	restoreMu sync.Mutex // 保护 restores，协调 InstallGlobal 与 Shutdown。
	restores  []func()   // 按安装顺序登记，Shutdown 时按 LIFO 恢复 global state。

	shutdown    sync.Once // 保证所有 Provider 的 Shutdown 最多执行一次。
	shutdownErr error     // 缓存首次 Shutdown 结果，后续调用返回同一错误。
}

// runtimeCounters 保存可无锁读取的进程内诊断计数。
// atomic 只保护计数更新，不参与 OTLP 记录发送或批处理。
type runtimeCounters struct {
	logExportErrors atomic.Uint64
}

// RuntimeStats 是 Runtime 的导出诊断快照。它不代表后端已持久化，只记录 SDK
// exporter 返回的错误次数；日志队列丢弃仍由 OTel SDK 的内部诊断日志报告。
type RuntimeStats struct {
	// LogExportErrors 是 OTLP Log Exporter 或 LoggerProvider Shutdown 返回错误的累计次数。
	LogExportErrors uint64
}

// Stats 返回当前 Runtime 的导出错误快照，可由多个 goroutine 并发调用。
func (r *Runtime) Stats() RuntimeStats {
	if r == nil || r.counters == nil {
		return RuntimeStats{}
	}
	return RuntimeStats{LogExportErrors: r.counters.logExportErrors.Load()}
}

// Tracer 返回当前 Runtime 的 Tracer；Runtime 为 nil 或未配置时回退到进程级
// global TracerProvider。返回的 Tracer 遵循 OTel API 的并发契约。
func (r *Runtime) Tracer(name string) trace.Tracer {
	if r == nil || r.tracerProvider == nil {
		return otel.Tracer(name)
	}
	return r.tracerProvider.Tracer(name)
}

// Meter 返回当前 Runtime 的 Meter；Runtime 为 nil 或未配置时回退到进程级
// global MeterProvider。返回的 Meter 遵循 OTel API 的并发契约。
func (r *Runtime) Meter(name string) metric.Meter {
	if r == nil || r.meterProvider == nil {
		return otel.Meter(name)
	}
	return r.meterProvider.Meter(name)
}
