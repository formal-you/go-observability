// Command config 演示应用如何解析 app.example.yaml，并映射到
// telemetry.Config 与 log.NewLogger options。
//
// 这是应用层示例：库本身不会读取 YAML。运行方式：
//
//	cd example/16_config
//	go run .
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/logwriter/file"
	"github.com/formal-you/go-observability/telemetry"
)

// AppConfig 是应用自己的配置根。
type AppConfig struct {
	Observability ObservabilityConfig `yaml:"observability"`
}

// ObservabilityConfig 对应 app.example.yaml 的 observability 段。
type ObservabilityConfig struct {
	Enabled    bool           `yaml:"enabled"`
	Endpoint   string         `yaml:"endpoint"`
	Resource   ResourceConfig `yaml:"resource"`
	Trace      TraceConfig    `yaml:"trace"`
	Metric     MetricConfig   `yaml:"metric"`
	Log        LogConfig      `yaml:"log"`
	LogOptions LogOptions     `yaml:"log_options"`
}

// ResourceConfig 映射 telemetry.ResourceConfig。
type ResourceConfig struct {
	ServiceName    string `yaml:"service_name"`
	ServiceVersion string `yaml:"service_version"`
	Environment    string `yaml:"environment"`
	Region         string `yaml:"region"`
	Instance       string `yaml:"instance"`
}

// TraceConfig 映射 telemetry.TraceConfig。
type TraceConfig struct {
	Output       string  `yaml:"output"`
	FilePath     string  `yaml:"file_path"`
	SampleRatio  float64 `yaml:"sample_ratio"`
	BatchTimeout string  `yaml:"batch_timeout"`
}

// MetricConfig 映射 telemetry.MetricConfig。
type MetricConfig struct {
	Output         string `yaml:"output"`
	FilePath       string `yaml:"file_path"`
	ExportInterval string `yaml:"export_interval"`
}

// LogConfig 映射 telemetry.LogConfig；rotation 是应用层字段。
type LogConfig struct {
	Output       string         `yaml:"output"`
	FilePath     string         `yaml:"file_path"`
	BatchTimeout string         `yaml:"batch_timeout"`
	QueueSize    int            `yaml:"queue_size"`
	Rotation     RotationConfig `yaml:"rotation"`
}

// RotationConfig 解析后转为 file.WithRotation。
type RotationConfig struct {
	MaxSizeMB  int  `yaml:"max_size_mb"`
	MaxBackups int  `yaml:"max_backups"`
	MaxAgeDays int  `yaml:"max_age_days"`
	Compress   bool `yaml:"compress"`
	LocalTime  bool `yaml:"local_time"`
}

// LogOptions 对应 log.NewLogger 的治理选项。
type LogOptions struct {
	MinLevel string        `yaml:"min_level"`
	Sampler  SamplerConfig `yaml:"sampler"`
	Masker   MaskerConfig  `yaml:"masker"`
}

// SamplerConfig 描述采样配置。
type SamplerConfig struct {
	Type          string   `yaml:"type"` // none | event_keep | result_keep
	KeepTypes     []string `yaml:"keep_types"`
	FallbackRatio float64  `yaml:"fallback_ratio"`
	Ratio         float64  `yaml:"ratio"`
}

// MaskerConfig 描述脱敏配置。
type MaskerConfig struct {
	Enabled   bool     `yaml:"enabled"`
	ExtraKeys []string `yaml:"extra_keys"`
	Redact    string   `yaml:"redact"`
}

func main() {
	data, err := os.ReadFile("app.example.yaml")
	if err != nil {
		panic(err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		panic(err)
	}

	tc, err := buildTelemetryConfig(cfg.Observability)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	runtime, err := telemetry.NewRuntime(ctx, tc)
	if err != nil {
		panic(err)
	}
	defer runtime.Shutdown(ctx)

	restore := runtime.InstallGlobal()
	defer restore()

	writer, err := runtime.NewLogWriter(ctx)
	if err != nil {
		panic(err)
	}
	defer writer.Close(ctx)

	logger := log.NewLogger(writer, buildLoggerOptions(cfg.Observability.LogOptions)...)
	_ = logger
	fmt.Println("telemetry runtime assembled from app.example.yaml")
}

func buildTelemetryConfig(oc ObservabilityConfig) (telemetry.Config, error) {
	var logFileOptions []file.Option
	if oc.Log.Rotation.MaxSizeMB > 0 {
		logFileOptions = append(logFileOptions, file.WithRotation(file.RotationConfig{
			MaxSizeMB:  oc.Log.Rotation.MaxSizeMB,
			MaxBackups: oc.Log.Rotation.MaxBackups,
			MaxAgeDays: oc.Log.Rotation.MaxAgeDays,
			Compress:   oc.Log.Rotation.Compress,
			LocalTime:  oc.Log.Rotation.LocalTime,
		}))
	}

	logBatch, err := duration(oc.Log.BatchTimeout, time.Second)
	if err != nil {
		return telemetry.Config{}, err
	}
	traceBatch, err := duration(oc.Trace.BatchTimeout, 5*time.Second)
	if err != nil {
		return telemetry.Config{}, err
	}
	metricInterval, err := duration(oc.Metric.ExportInterval, 15*time.Second)
	if err != nil {
		return telemetry.Config{}, err
	}

	return telemetry.Config{
		Enabled:  oc.Enabled,
		Endpoint: oc.Endpoint,
		Resource: telemetry.ResourceConfig{
			ServiceName:    oc.Resource.ServiceName,
			ServiceVersion: oc.Resource.ServiceVersion,
			Environment:    oc.Resource.Environment,
			Region:         oc.Resource.Region,
			Instance:       oc.Resource.Instance,
		},
		Trace: telemetry.TraceConfig{
			Output:       telemetry.SignalOutput(oc.Trace.Output),
			FilePath:     oc.Trace.FilePath,
			SampleRatio:  oc.Trace.SampleRatio,
			BatchTimeout: traceBatch,
		},
		Metric: telemetry.MetricConfig{
			Output:         telemetry.SignalOutput(oc.Metric.Output),
			FilePath:       oc.Metric.FilePath,
			ExportInterval: metricInterval,
		},
		Log: telemetry.LogConfig{
			Output:       telemetry.SignalOutput(oc.Log.Output),
			FilePath:     oc.Log.FilePath,
			FileOptions:  logFileOptions,
			BatchTimeout: logBatch,
			QueueSize:    oc.Log.QueueSize,
		},
	}, nil
}

func buildLoggerOptions(lo LogOptions) []log.Option {
	var opts []log.Option

	if lo.MinLevel != "" {
		opts = append(opts, log.WithMinLevel(log.Level(lo.MinLevel)))
	}

	if lo.Masker.Enabled {
		opts = append(opts, log.WithMasker(log.FieldMasker{
			Keys:   lo.Masker.ExtraKeys,
			Redact: lo.Masker.Redact,
		}))
	}

	if sampler := samplerFrom(lo.Sampler); sampler != nil {
		opts = append(opts, log.WithSampler(sampler))
	}

	return opts
}

func samplerFrom(sc SamplerConfig) log.Sampler {
	switch sc.Type {
	case "", "none":
		return nil
	case "result_keep":
		return log.NewResultKeepSampler(ratioOr(sc.Ratio, 1.0))
	case "event_keep":
		types := make([]log.EventType, 0, len(sc.KeepTypes))
		for _, t := range sc.KeepTypes {
			types = append(types, log.EventType(t))
		}
		return log.NewEventTypeKeepSampler(types, log.NewResultKeepSampler(ratioOr(sc.FallbackRatio, 0.1)))
	default:
		panic("unknown sampler type: " + sc.Type)
	}
}

func ratioOr(value, fallback float64) float64 {
	if value <= 0 || value > 1 {
		return fallback
	}
	return value
}

func duration(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}
