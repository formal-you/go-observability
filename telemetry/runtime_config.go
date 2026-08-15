package telemetry

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/formal-you/go-observability/internal/otlpendpoint"
	"github.com/formal-you/go-observability/writer/file"
)

// 默认值属于 SDK 装配策略，不等同于 Collector 的 batch 或日志事件 sampling。
const defaultEndpoint = "127.0.0.1:4317"

const (
	defaultTraceBatchTimeout    = 5 * time.Second
	defaultMetricExportInterval = 15 * time.Second
	defaultLogBatchTimeout      = 1 * time.Second
	defaultTraceSampleRatio     = 0.1
	defaultExportBatchSize      = 512
	defaultLogQueueSize         = 2048
)

// normalizeAndValidateConfig 归一化并校验完整 Config；disabled 走独立路径。
func normalizeAndValidateConfig(cfg *Config) error {
	if !cfg.Enabled {
		return normalizeDisabledConfig(cfg)
	}
	if strings.TrimSpace(cfg.Resource.ServiceName) == "" {
		return errors.New("telemetry: service name is required")
	}
	if err := normalizeAndValidateLog(&cfg.Log); err != nil {
		return err
	}
	if err := normalizeAndValidateTrace(&cfg.Trace); err != nil {
		return err
	}
	if err := normalizeAndValidateMetric(&cfg.Metric); err != nil {
		return err
	}
	if cfg.Resource.Environment == "" {
		cfg.Resource.Environment = "development"
	}
	return nil
}

// normalizeDisabledConfig 保留日志 Writer 能力，但不创建任何 Provider。
// 与旧兼容行为一致：Log.Output 为空或 otlp 时归一化为 file，便于本地 JSONL 开发。
func normalizeDisabledConfig(cfg *Config) error {
	if cfg.Log.Output == "" || cfg.Log.Output == SignalOutputOTLP {
		cfg.Log.Output = SignalOutputFile
	}
	if err := validateSignalOutput(cfg.Log.Output, "log", false); err != nil {
		return err
	}
	switch cfg.Log.Output {
	case SignalOutputFile:
		if strings.TrimSpace(cfg.Log.FilePath) == "" {
			return errors.New("telemetry: file output requires file path")
		}
	case SignalOutputStdout:
		if cfg.Log.FilePath != "" || len(cfg.Log.FileOptions) != 0 {
			return errors.New("telemetry: file options do not apply to stdout output")
		}
	case SignalOutputNone:
		if cfg.Log.FilePath != "" || len(cfg.Log.FileOptions) != 0 || len(cfg.Log.StdoutOptions) != 0 {
			return errors.New("telemetry: writer options do not apply to none output")
		}
	}
	return nil
}

// normalizeAndValidateLog 归一化并校验 Log 信号配置及 Writer 选项。
func normalizeAndValidateLog(cfg *LogConfig) error {
	if err := validateSignalOutput(cfg.Output, "log", false); err != nil {
		return err
	}
	switch cfg.Output {
	case SignalOutputFile:
		if strings.TrimSpace(cfg.FilePath) == "" {
			return errors.New("telemetry: file output requires file path")
		}
		if len(cfg.StdoutOptions) != 0 {
			return errors.New("telemetry: stdout options do not apply to file output")
		}
	case SignalOutputOTLP:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 || len(cfg.StdoutOptions) != 0 {
			return errors.New("telemetry: writer options do not apply to otlp output")
		}
	case SignalOutputStdout:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 {
			return errors.New("telemetry: file options do not apply to stdout output")
		}
	case SignalOutputNone:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 || len(cfg.StdoutOptions) != 0 {
			return errors.New("telemetry: writer options do not apply to none output")
		}
	default:
		return fmt.Errorf("telemetry: invalid log output %q", cfg.Output)
	}
	if err := normalizePositive(&cfg.BatchTimeout, defaultLogBatchTimeout, "log batch timeout"); err != nil {
		return err
	}
	return normalizePositive(&cfg.QueueSize, defaultLogQueueSize, "log queue size")
}

// normalizeAndValidateTrace 归一化并校验 Trace 信号配置，含 local 与注入 Exporter 边界。
func normalizeAndValidateTrace(cfg *TraceConfig) error {
	if cfg.Output == "" {
		cfg.Output = SignalOutputOTLP
	}
	if err := validateSignalOutput(cfg.Output, "trace", true); err != nil {
		return err
	}
	switch cfg.Output {
	case SignalOutputFile:
		if strings.TrimSpace(cfg.FilePath) == "" {
			return errors.New("telemetry: trace file output requires trace_file path")
		}
	case SignalOutputLocal, SignalOutputNone:
		if cfg.FilePath != "" {
			return fmt.Errorf("telemetry: trace file path does not apply to %s output", cfg.Output)
		}
		if cfg.Exporter != nil {
			return fmt.Errorf("telemetry: trace exporter does not apply to %s output", cfg.Output)
		}
	default:
		if cfg.FilePath != "" {
			return errors.New("telemetry: trace file path does not apply to selected output")
		}
	}
	if cfg.Output != SignalOutputOTLP && cfg.Exporter != nil {
		return fmt.Errorf("telemetry: trace exporter does not apply to %s output", cfg.Output)
	}
	if cfg.SampleRatio == 0 {
		cfg.SampleRatio = defaultTraceSampleRatio
	}
	if cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return errors.New("telemetry: trace sample ratio must be in (0, 1]")
	}
	return normalizePositive(&cfg.BatchTimeout, defaultTraceBatchTimeout, "trace batch timeout")
}

// normalizeAndValidateMetric 归一化并校验 Metric 信号配置，local 仅限 Trace。
func normalizeAndValidateMetric(cfg *MetricConfig) error {
	if cfg.Output == "" {
		cfg.Output = SignalOutputOTLP
	}
	if err := validateSignalOutput(cfg.Output, "metric", true); err != nil {
		return err
	}
	if cfg.Output == SignalOutputLocal {
		return errors.New("telemetry: local output is only valid for trace")
	}
	switch cfg.Output {
	case SignalOutputFile:
		if strings.TrimSpace(cfg.FilePath) == "" {
			return errors.New("telemetry: metric file output requires metric_file path")
		}
	default:
		if cfg.FilePath != "" {
			return errors.New("telemetry: metric file path does not apply to selected output")
		}
	}
	if cfg.Output != SignalOutputOTLP && cfg.Reader != nil {
		return fmt.Errorf("telemetry: metric reader does not apply to %s output", cfg.Output)
	}
	return normalizePositive(&cfg.ExportInterval, defaultMetricExportInterval, "metric export interval")
}

// validateSignalOutput 校验信号输出枚举；allowLocal 为 true 时才接受 Trace 专用 local。
func validateSignalOutput(output SignalOutput, name string, allowLocal bool) error {
	switch output {
	case SignalOutputFile, SignalOutputOTLP, SignalOutputStdout, SignalOutputNone:
		return nil
	case SignalOutputLocal:
		if allowLocal {
			return nil
		}
		return fmt.Errorf("telemetry: local output is not valid for %s", name)
	default:
		return fmt.Errorf("telemetry: invalid %s output %q", name, output)
	}
}

// normalizePositive 为零值字段应用默认值并拒绝负数；duration 与整型队列容量共用。
func normalizePositive[T int | time.Duration](value *T, def T, name string) error {
	if *value == 0 {
		*value = def
	}
	if *value < 0 {
		return fmt.Errorf("telemetry: %s must be positive", name)
	}
	return nil
}

// resourceForConfig 从 ResourceConfig 构建 OTel Resource；Override 优先于服务身份字段。
func resourceForConfig(cfg ResourceConfig) *resource.Resource {
	// 显式 Resource 是高级注入点：一旦提供，就由调用方承担其属性完整性。
	if cfg.Override != nil {
		return cfg.Override
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

// resourceMetadata 把 OTel Resource 投影为 file Writer 的进程级服务身份元数据。
func resourceMetadata(res *resource.Resource) file.ResourceMetadata {
	// file Writer 没有 OTel Resource 层，因此只投影稳定的进程级服务身份键。
	// 其他 Resource Attributes 不复制到每条 JSONL，避免扩大文件日志字段面。
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

// EnabledFromEnvironment 读取 OTEL_SDK_DISABLED 环境变量并映射为 Config.Enabled。
// 环境变量值忽略首尾空白和大小写；只有 true 表示禁用。
func EnabledFromEnvironment() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true")
}

// EndpointFromEnvironment 读取 OTEL_EXPORTER_OTLP_ENDPOINT；未设置时返回本地默认地址。
func EndpointFromEnvironment() string {
	return defaultIfEmpty(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")), defaultEndpoint)
}

// endpointURL 统一解析并规范化 OTLP endpoint，避免不同 Exporter 各自回退。
func endpointURL(endpoint string) (string, error) {
	// 所有 OTLP signal 共用统一解析规则，避免不同 Exporter 对非法地址各自回退。
	normalized, err := otlpendpoint.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("telemetry: %w", err)
	}
	return normalized, nil
}

// defaultIfEmpty 在 value 为空时返回 fallback。
func defaultIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// normalizedTraceOutput 把空 Trace Output 归一化为 OTLP。
func normalizedTraceOutput(o SignalOutput) SignalOutput {
	if o == "" {
		return SignalOutputOTLP
	}
	return o
}

// normalizedMetricOutput 把空 Metric Output 归一化为 OTLP。
func normalizedMetricOutput(o SignalOutput) SignalOutput {
	if o == "" {
		return SignalOutputOTLP
	}
	return o
}

// otlpEndpointRequired 报告是否有信号会在 NewRuntime 中实际创建 OTLP exporter/reader。
// 注入自定义 Exporter/Reader 时，对应信号不消费 Endpoint。
func otlpEndpointRequired(cfg Config) bool {
	if normalizedTraceOutput(cfg.Trace.Output) == SignalOutputOTLP && cfg.Trace.Exporter == nil {
		return true
	}
	if normalizedMetricOutput(cfg.Metric.Output) == SignalOutputOTLP && cfg.Metric.Reader == nil {
		return true
	}
	return cfg.Log.Output == SignalOutputOTLP
}
