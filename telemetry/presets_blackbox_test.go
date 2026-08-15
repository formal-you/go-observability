package telemetry_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	obslog "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/telemetry"
)

// TestNewLogRuntimeFileWritesServiceIdentity 验证 Log-only 快捷构造在 file 出口下
// 创建 ManagedWriter 并注入 service.name。
func TestNewLogRuntimeFileWritesServiceIdentity(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	runtime, err := telemetry.NewLogRuntime(ctx, "log-only", telemetry.SignalOutputFile, path)
	if err != nil {
		t.Fatalf("NewLogRuntime: %v", err)
	}
	defer func() { _ = runtime.Shutdown(ctx) }()

	writer, err := runtime.NewWriter(ctx)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := writer.Write(ctx, "business", slog.String(string(obslog.KeyEventName), "order.created")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(ctx); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Fatalf("events file: err=%v data=%q", err, string(data))
	}
	var record map[string]any
	if err := json.Unmarshal(data[:len(data)-1], &record); err != nil {
		t.Fatal(err)
	}
	if record["service.name"] != "log-only" {
		t.Fatalf("service.name = %v, want log-only", record["service.name"])
	}
}

// TestNewLogRuntimeStdoutLifecycle 验证 Log-only 快捷构造在 stdout 出口下可正常创建并关闭 Writer。
func TestNewLogRuntimeStdoutLifecycle(t *testing.T) {
	ctx := context.Background()
	runtime, err := telemetry.NewLogRuntime(ctx, "log-only", telemetry.SignalOutputStdout, "")
	if err != nil {
		t.Fatalf("NewLogRuntime: %v", err)
	}
	defer func() { _ = runtime.Shutdown(ctx) }()

	writer, err := runtime.NewWriter(ctx)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := writer.Write(ctx, "probe"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestNewLogRuntimeValidation 验证 Log-only 快捷构造对必填参数和互斥参数的校验。
func TestNewLogRuntimeValidation(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		service  string
		output   telemetry.SignalOutput
		filePath string
	}{
		{name: "missing service", output: telemetry.SignalOutputFile, filePath: "x"},
		{name: "invalid output", service: "svc", output: telemetry.SignalOutputOTLP},
		{name: "file missing path", service: "svc", output: telemetry.SignalOutputFile},
		{name: "stdout with path", service: "svc", output: telemetry.SignalOutputStdout, filePath: "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := telemetry.NewLogRuntime(ctx, tc.service, tc.output, tc.filePath); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

// TestNewOTLPRuntimeValidationAndLifecycle 验证全 OTLP 快捷构造的必填参数与关闭行为。
// 本测试不依赖真实 Collector：构造成功后只验证 Runtime 生命周期，忽略 Shutdown 的
// 网络上传错误（真实 OTLP 行为由 otlp_reliability_blackbox_test 覆盖）。
func TestNewOTLPRuntimeValidationAndLifecycle(t *testing.T) {
	ctx := context.Background()
	if _, err := telemetry.NewOTLPRuntime(ctx, "", "127.0.0.1:4317"); err == nil {
		t.Fatal("missing service name: want error")
	}
	if _, err := telemetry.NewOTLPRuntime(ctx, "svc", ""); err == nil {
		t.Fatal("missing endpoint: want error")
	}

	runtime, err := telemetry.NewOTLPRuntime(ctx, "otlp-preset", "127.0.0.1:4317")
	if err != nil {
		t.Fatalf("NewOTLPRuntime: %v", err)
	}
	defer func() { _ = runtime.Shutdown(context.Background()) }()
}

// TestNewAllFileRuntimeCreatesThreeFiles 验证全文件快捷构造创建三个信号文件并注入服务身份。
// Log 文件由 NewWriter 懒创建；Trace/Metric 文件在构造时创建。
func TestNewAllFileRuntimeCreatesThreeFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	runtime, err := telemetry.NewAllFileRuntime(ctx, "all-file", dir)
	if err != nil {
		t.Fatalf("NewAllFileRuntime: %v", err)
	}
	defer func() { _ = runtime.Shutdown(ctx) }()

	for _, name := range []string{"trace.jsonl", "metric.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}

	writer, err := runtime.NewWriter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(ctx, "business", slog.String(string(obslog.KeyEventName), "order.created")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(ctx); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil || len(data) == 0 {
		t.Fatalf("events file: err=%v data=%q", err, string(data))
	}
	var record map[string]any
	if err := json.Unmarshal(data[:len(data)-1], &record); err != nil {
		t.Fatal(err)
	}
	if record["service.name"] != "all-file" {
		t.Fatalf("service.name = %v, want all-file", record["service.name"])
	}
}

// TestNewAllFileRuntimeValidation 验证全文件快捷构造的必填参数校验。
func TestNewAllFileRuntimeValidation(t *testing.T) {
	ctx := context.Background()
	if _, err := telemetry.NewAllFileRuntime(ctx, "", "logs"); err == nil {
		t.Fatal("missing service name: want error")
	}
	if _, err := telemetry.NewAllFileRuntime(ctx, "svc", ""); err == nil {
		t.Fatal("missing dir: want error")
	}
}
