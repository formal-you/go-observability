package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// TestRuntimeOutputNoneDisablesTraceMetric 验证 TraceOutput/MetricOutput=none 时不装配
// 对应 Provider：Tracer/Meter 回退进程级 no-op，InstallGlobal/Shutdown 安全。
func TestRuntimeOutputNoneDisablesTraceMetric(t *testing.T) {
	ctx := context.Background()
	r, err := NewRuntime(ctx, Config{
		ServiceName:  "output-none",
		Enabled:      true,
		LogOutput:    LogOutputNone,
		TraceOutput:  LogOutputNone,
		MetricOutput: LogOutputNone,
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

// TestRuntimeFileOutputsWriteTraceMetricFiles 验证 TraceOutput/MetricOutput=file 时
// Trace/Metric 落到指定文件（stdout exporters + WithWriter，参考 example/otel）。
func TestRuntimeFileOutputsWriteTraceMetricFiles(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "traces.json")
	metricPath := filepath.Join(dir, "metrics.json")
	ctx := context.Background()
	r, err := NewRuntime(ctx, Config{
		ServiceName:          "output-file",
		Enabled:              true,
		LogOutput:            LogOutputNone,
		TraceOutput:          LogOutputFile,
		TraceFile:            tracePath,
		MetricOutput:         LogOutputFile,
		MetricFile:           metricPath,
		TraceSampleRatio:     1.0,
		TraceBatchTimeout:    100 * time.Millisecond,
		MetricExportInterval: 100 * time.Millisecond,
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

// TestRuntimeOutputValidation 验证分信号输出的配置校验：非法值 / file 缺路径。
func TestRuntimeOutputValidation(t *testing.T) {
	ctx := context.Background()
	if _, err := NewRuntime(ctx, Config{ServiceName: "x", Enabled: true, TraceOutput: "bogus"}); err == nil {
		t.Fatal("invalid trace output should error")
	}
	if _, err := NewRuntime(ctx, Config{ServiceName: "x", Enabled: true, MetricOutput: "bogus"}); err == nil {
		t.Fatal("invalid metric output should error")
	}
	if _, err := NewRuntime(ctx, Config{ServiceName: "x", Enabled: true, TraceOutput: LogOutputFile}); err == nil {
		t.Fatal("trace file output without path should error")
	}
	if _, err := NewRuntime(ctx, Config{ServiceName: "x", Enabled: true, MetricOutput: LogOutputFile}); err == nil {
		t.Fatal("metric file output without path should error")
	}
}
