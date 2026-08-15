package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// TestRuntimeOutputNoneDisablesTraceMetric 验证 Trace/Metric Output=none 时不装配
// 对应 Provider：Tracer/Meter 回退进程级 no-op，InstallGlobal/Shutdown 安全。
func TestRuntimeOutputNoneDisablesTraceMetric(t *testing.T) {
	ctx := context.Background()
	r, err := NewRuntime(ctx, Config{
		Enabled:  true,
		Resource: ResourceConfig{ServiceName: "output-none"},
		Trace:    TraceConfig{Output: SignalOutputNone},
		Metric:   MetricConfig{Output: SignalOutputNone},
		Log:      LogConfig{Output: SignalOutputNone},
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	restore := r.InstallGlobal()
	if r.Tracer("t") == nil || r.Meter("m") == nil {
		t.Fatal("Tracer/Meter should fall back to non-nil no-op")
	}
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	restore()
}

// TestRuntimeFileOutputsWriteTraceMetricFiles 验证 Trace/Metric Output=file 时
// Trace/Metric 落到指定文件（stdout exporters + WithWriter，参考 example/14_otel_logs）。
func TestRuntimeFileOutputsWriteTraceMetricFiles(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "traces.json")
	metricPath := filepath.Join(dir, "metrics.json")
	ctx := context.Background()
	r, err := NewRuntime(ctx, Config{
		Enabled:  true,
		Resource: ResourceConfig{ServiceName: "output-file"},
		Trace: TraceConfig{
			Output:       SignalOutputFile,
			FilePath:     tracePath,
			SampleRatio:  1.0,
			BatchTimeout: 100 * time.Millisecond,
		},
		Metric: MetricConfig{
			Output:         SignalOutputFile,
			FilePath:       metricPath,
			ExportInterval: 100 * time.Millisecond,
		},
		Log: LogConfig{Output: SignalOutputNone},
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	restore := r.InstallGlobal()

	// 记录一个 span。
	_, span := r.Tracer("test").Start(ctx, "output-modes-op")
	span.SetAttributes(attribute.String("k", "v"))
	span.End()

	// 记录一个 counter 增量。
	counter, err := r.Meter("test").Int64Counter("output.modes.counter")
	if err != nil {
		t.Fatalf("create counter: %v", err)
	}
	counter.Add(ctx, 1)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	restore()

	if data, err := os.ReadFile(tracePath); err != nil || len(data) == 0 {
		t.Fatalf("trace file empty/missing: err=%v data=%q", err, string(data))
	}
	if data, err := os.ReadFile(metricPath); err != nil || len(data) == 0 {
		t.Fatalf("metric file empty/missing: err=%v data=%q", err, string(data))
	}
}

// TestRuntimeLocalTraceDoesNotExport 验证 Trace.Output=local 不写文件也不要求 FilePath。
func TestRuntimeLocalTraceDoesNotExport(t *testing.T) {
	ctx := context.Background()
	r, err := NewRuntime(ctx, Config{
		Enabled:  true,
		Resource: ResourceConfig{ServiceName: "local-trace"},
		Trace:    TraceConfig{Output: SignalOutputLocal},
		Metric:   MetricConfig{Output: SignalOutputNone},
		Log:      LogConfig{Output: SignalOutputNone},
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	defer r.Shutdown(ctx)
	tracer := r.Tracer("test")
	_, span := tracer.Start(ctx, "local")
	if !span.SpanContext().TraceID().IsValid() || span.SpanContext().IsSampled() {
		t.Fatalf("local trace should generate valid unsampled span, got %s", span.SpanContext().TraceID())
	}
	span.End()
}

// TestRuntimeOutputValidation 验证分信号输出的配置校验：非法值 / file 缺路径 / local 误用。
func TestRuntimeOutputValidation(t *testing.T) {
	ctx := context.Background()
	base := func(mutate func(*Config)) Config {
		cfg := Config{
			Enabled:  true,
			Resource: ResourceConfig{ServiceName: "x"},
			Trace:    TraceConfig{Output: SignalOutputNone},
			Metric:   MetricConfig{Output: SignalOutputNone},
			Log:      LogConfig{Output: SignalOutputNone},
		}
		mutate(&cfg)
		return cfg
	}
	tests := []Config{
		base(func(cfg *Config) { cfg.Trace.Output = "bogus" }),
		base(func(cfg *Config) { cfg.Metric.Output = "bogus" }),
		base(func(cfg *Config) { cfg.Trace.Output = SignalOutputFile }),
		base(func(cfg *Config) { cfg.Metric.Output = SignalOutputFile }),
		base(func(cfg *Config) { cfg.Metric.Output = SignalOutputLocal }),
		base(func(cfg *Config) { cfg.Log.Output = SignalOutputFile }),
	}
	for _, cfg := range tests {
		if _, err := NewRuntime(ctx, cfg); err == nil {
			t.Errorf("NewRuntime(%+v) should error", cfg)
		}
	}
}
