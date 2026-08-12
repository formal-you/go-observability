// Package telemetry 为应用装配 OpenTelemetry Trace、Metric 和 Log Provider。
//
// Runtime 是本包的资源所有权边界：它持有由 Config 创建的 Provider，并负责按
// Log -> Metric -> Trace 的顺序关闭。NewRuntime 只构造独立 Runtime，不修改进程级
// OpenTelemetry global state；需要让依赖全局 API 的组件使用这些 Provider 时，调用
// Runtime.InstallGlobal，并在退出时调用其 restore 函数。
//
// 日志出口由 Config.LogOutput 显式选择。OTLP 出口把并发入队、批量导出和 gRPC 连接
// 交给 OTel SDK；file 出口则由文件 Writer 自行同步并发写入和轮转。
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

// LogOutput 表示 Runtime.NewWriter 创建的日志出口类型。
type LogOutput string

const (
	// LogOutputFile 把日志写为本地 JSONL 文件。
	LogOutputFile LogOutput = "file"
	// LogOutputOTLP 通过 OTel SDK 的 OTLP/gRPC Exporter 把日志发送到 Collector。
	LogOutputOTLP LogOutput = "otlp"
	// LogOutputStdout 使用 OTel stdout Exporter 输出日志，适合本地开发和诊断。
	LogOutputStdout LogOutput = "stdout"
	// LogOutputNone 丢弃日志，但仍可使用 Runtime 中已配置的 Trace 和 Metric Provider。
	LogOutputNone LogOutput = "none"
)

// Config 定义当前进程 Telemetry Runtime 的装配参数。
//
// Config 只在构造 Runtime 时读取；构造完成后修改 Config 不会动态更新 Provider。
// Enabled=true 时 ServiceName 和 LogOutput 必填。时间字段表示 SDK 侧的批量导出周期，
// 不是 sampling，也不是 Collector 的 batch processor 配置。
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
	// Endpoint 配置 OTLP/gRPC Collector 地址；为空时使用 127.0.0.1:4317。
	// 它不用于推断 LogOutput，新代码必须显式选择出口。
	Endpoint string
	// Enabled 控制是否装配 Telemetry Provider；false 返回不含 Provider 的空 Runtime。
	Enabled bool
	// LogOutput 显式选择 file、otlp、stdout 或 none。
	LogOutput LogOutput
	// TraceSampleRatio 是 SDK head sampling 比例；零值使用 0.1。
	// 未被采样的 Trace 不会发送到 Collector，Collector tail sampling 无法恢复。
	TraceSampleRatio float64
	// TraceBatchTimeout 是 SpanProcessor 的 SDK 批量导出间隔；零值使用 5s。
	TraceBatchTimeout time.Duration
	// MetricExportInterval 是 Metric Reader 的 SDK 周期导出间隔；零值使用 15s。
	MetricExportInterval time.Duration
	// LogBatchTimeout 是 Log BatchProcessor 的 SDK 批量导出间隔；零值使用 1s。
	LogBatchTimeout time.Duration
	// LogQueueSize 是 OTLP Log BatchProcessor 的有界队列容量；零值使用 2048。
	// 队列满时的丢弃由 OTel SDK 诊断机制报告，不由 Logger.Emit 返回给调用方。
	LogQueueSize int
	// TraceExporter 允许测试或自定义部署注入公开 OTel SpanExporter。
	// 为空时按 Endpoint 创建 OTLP gRPC Exporter。成功装配后，它由 TracerProvider
	// 持有并随 Runtime.Shutdown 关闭；构造失败时，注入对象不由本函数回滚关闭。
	TraceExporter sdktrace.SpanExporter
	// MetricReader 允许测试或自定义部署注入公开 OTel Reader。
	// 为空时按 Endpoint 创建 OTLP PeriodicReader。成功装配后，它由 MeterProvider
	// 持有并随 Runtime.Shutdown 关闭；构造失败时，注入对象不由本函数回滚关闭。
	MetricReader sdkmetric.Reader
	// Resource 覆盖由服务身份字段构建的 OTel Resource；注入对象由调用方拥有。
	Resource *resource.Resource
}

// WriterConfig 包含 Runtime.NewWriter 所选日志出口的参数。
// 与当前 LogOutput 不匹配的选项会返回错误，不会被静默忽略。
type WriterConfig struct {
	// FilePath 是 LogOutputFile 的 JSONL 路径，其他出口必须留空。
	FilePath string
	// FileOptions 只用于 LogOutputFile；Runtime 会在其后注入自身 Resource metadata。
	FileOptions []file.Option
	// StdoutOptions 只用于 LogOutputStdout。
	StdoutOptions []stdoutwriter.Option
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
	logOutput      LogOutput
	fileMetadata   file.ResourceMetadata
	counters       *runtimeCounters

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

// Providers 是为源代码兼容保留的 Runtime 类型别名。
// Deprecated: 使用 Runtime。
type Providers = Runtime

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

// Resource 返回 Runtime 的 Provider 与 Writer 共享的 OTel Resource。
// 调用方应把返回值视为只读，不负责关闭。
func (r *Runtime) Resource() *resource.Resource {
	if r == nil {
		return nil
	}
	return r.resource
}

// LoggerProvider 返回 Runtime 的 OTLP LoggerProvider；非 OTLP 日志出口返回 nil。
// Provider 由 Runtime 拥有，调用方不得单独 Shutdown。
func (r *Runtime) LoggerProvider() *sdklog.LoggerProvider {
	if r == nil {
		return nil
	}
	return r.loggerProvider
}
