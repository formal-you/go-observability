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
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return errors.New("telemetry: service name is required")
	}
	switch cfg.LogOutput {
	case LogOutputFile, LogOutputOTLP, LogOutputStdout, LogOutputNone:
	default:
		return fmt.Errorf("telemetry: invalid log output %q", cfg.LogOutput)
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

// EnabledFromEnvironment reads the legacy SDK disabled switch.
func EnabledFromEnvironment() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true")
}

// EndpointFromEnvironment reads the legacy OTLP endpoint or returns the local default.
func EndpointFromEnvironment() string {
	return defaultIfEmpty(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")), defaultEndpoint)
}

func endpointURL(endpoint string) (string, error) {
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
