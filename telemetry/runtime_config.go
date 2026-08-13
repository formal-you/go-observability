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

func normalizeAndValidateConfig(cfg *Config) error {
	// 先验证身份与出口，再补齐默认值，确保配置错误在创建任何 Exporter 前返回。
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return errors.New("telemetry: service name is required")
	}
	switch cfg.LogOutput {
	case LogOutputFile, LogOutputOTLP, LogOutputStdout, LogOutputNone:
	default:
		return fmt.Errorf("telemetry: invalid log output %q", cfg.LogOutput)
	}
	switch outputOf(cfg.TraceOutput) {
	case LogOutputOTLP, LogOutputFile, LogOutputStdout, LogOutputNone:
	default:
		return fmt.Errorf("telemetry: invalid trace output %q", cfg.TraceOutput)
	}
	switch outputOf(cfg.MetricOutput) {
	case LogOutputOTLP, LogOutputFile, LogOutputStdout, LogOutputNone:
	default:
		return fmt.Errorf("telemetry: invalid metric output %q", cfg.MetricOutput)
	}
	if outputOf(cfg.TraceOutput) == LogOutputFile && strings.TrimSpace(cfg.TraceFile) == "" {
		return errors.New("telemetry: trace file output requires trace_file path")
	}
	if outputOf(cfg.MetricOutput) == LogOutputFile && strings.TrimSpace(cfg.MetricFile) == "" {
		return errors.New("telemetry: metric file output requires metric_file path")
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
	if cfg.LogQueueSize == 0 {
		cfg.LogQueueSize = defaultLogQueueSize
	}
	if cfg.LogQueueSize < 0 {
		return errors.New("telemetry: log queue size must be positive")
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	return nil
}

func resourceForConfig(cfg Config) *resource.Resource {
	// 显式 Resource 是高级注入点：一旦提供，就由调用方承担其属性完整性。
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

// EnabledFromEnvironment 读取兼容入口使用的 OTEL_SDK_DISABLED 开关。
// 环境变量值忽略首尾空白和大小写；只有 true 表示禁用。
func EnabledFromEnvironment() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true")
}

// EndpointFromEnvironment 读取兼容入口使用的 OTLP endpoint；未设置时返回本地默认地址。
func EndpointFromEnvironment() string {
	return defaultIfEmpty(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")), defaultEndpoint)
}

func endpointURL(endpoint string) (string, error) {
	// 所有三种 OTLP signal 共用统一解析规则，避免不同 Exporter 对非法地址各自回退。
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

// outputOf 归一化信号输出目标：空值按 OTLP 处理（与历史行为一致，保持向后兼容）。
func outputOf(o LogOutput) LogOutput {
	if o == "" {
		return LogOutputOTLP
	}
	return o
}
