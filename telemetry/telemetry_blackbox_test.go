// Package telemetry_test 是 telemetry 的外部黑盒测试层（package telemetry_test）。
// 用例只通过公开 API 验证环境变量、出口选择与生命周期，不读取实现内部。
package telemetry_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/formal-you/go-observability/telemetry"
)

// TestEnabledFromEnvironmentBlackBox 验证启用开关。
func TestEnabledFromEnvironmentBlackBox(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	if telemetry.EnabledFromEnvironment() {
		t.Error("OTEL_SDK_DISABLED=true should disable")
	}
	t.Setenv("OTEL_SDK_DISABLED", "")
	if !telemetry.EnabledFromEnvironment() {
		t.Error("unset should enable")
	}
}

// TestEndpointFromEnvironmentBlackBox 验证 endpoint helper。
func TestEndpointFromEnvironmentBlackBox(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	if got := telemetry.EndpointFromEnvironment(); got != "collector:4317" {
		t.Errorf("endpoint = %q, want collector:4317", got)
	}
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if got := telemetry.EndpointFromEnvironment(); got != "127.0.0.1:4317" {
		t.Errorf("default endpoint = %q, want 127.0.0.1:4317", got)
	}
}

// TestNewRuntimeDisabledBlackBox 验证 disabled Runtime 不建 Provider，但仍可写文件。
func TestNewRuntimeDisabledBlackBox(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disabled.jsonl")
	r, err := telemetry.NewRuntime(context.Background(), telemetry.Config{
		Enabled: false,
		Log:     telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(context.Background())
	w, err := r.NewWriter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(context.Background(), "business"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Fatalf("disabled file writer should write JSONL: err=%v data=%q", err, data)
	}
}

// TestNewRuntimeOutputIsFrozenBlackBox 验证 NewRuntime 后修改环境变量不会改变已选出口。
func TestNewRuntimeOutputIsFrozenBlackBox(t *testing.T) {
	ctx := context.Background()
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	path := filepath.Join(t.TempDir(), "frozen.jsonl")
	r, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  true,
		Resource: telemetry.ResourceConfig{ServiceName: "file-svc"},
		Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputLocal},
		Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputNone},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(ctx)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	w, err := r.NewWriter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(ctx, "business"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data[:len(data)-1], &record); err != nil {
		t.Fatalf("invalid JSONL: %v", err)
	}
	if record["service.name"] != "file-svc" {
		t.Fatalf("file writer should keep service.name, got %v", record)
	}
}

// TestNewRuntimeOTLPWriterBlackBox 验证 OTLP Runtime 能创建托管 Writer 且 Close 不关闭 Provider。
func TestNewRuntimeOTLPWriterBlackBox(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  true,
		Resource: telemetry.ResourceConfig{ServiceName: "otlp-contract"},
		Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputNone},
		Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputNone},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputOTLP},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(ctx)
	w, err := r.NewWriter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(ctx); err != nil {
		t.Fatalf("close runtime-backed OTLP Writer: %v", err)
	}
}
