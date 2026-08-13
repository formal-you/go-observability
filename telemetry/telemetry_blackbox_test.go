// Package telemetry_test 是 telemetry 的外部黑盒测试层（package telemetry_test）。
// 用例只通过公开 API 验证环境变量与出口选择，不读取实现内部。
package telemetry_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/formal-you/go-observability/telemetry"
	"github.com/formal-you/go-observability/writer/file"
	"github.com/formal-you/go-observability/writer/otlp"
)

// TestEnabledFromEnvironmentBlackBox 按 CASE-B9-01 验证启用开关（Oracle: ACCEPT-B9-01）。
func TestEnabledFromEnvironmentBlackBox(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	if telemetry.EnabledFromEnvironment() {
		t.Error("OTEL_SDK_DISABLED=true 时应禁用（Oracle: ACCEPT-B9-01）")
	}
	t.Setenv("OTEL_SDK_DISABLED", "")
	if !telemetry.EnabledFromEnvironment() {
		t.Error("未设置时应默认启用（Oracle: ACCEPT-B9-01）")
	}
}

// TestEndpointFromEnvironmentBlackBox 按 CASE-B9-02/03 验证 endpoint（Oracle: ACCEPT-B9-02/03）。
func TestEndpointFromEnvironmentBlackBox(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	if got := telemetry.EndpointFromEnvironment(); got != "collector:4317" {
		t.Errorf("endpoint = %q, want collector:4317（Oracle: ACCEPT-B9-02）", got)
	}
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if got := telemetry.EndpointFromEnvironment(); got != "127.0.0.1:4317" {
		t.Errorf("缺省 endpoint = %q, want 127.0.0.1:4317（Oracle: ACCEPT-B9-03）", got)
	}
}

// TestSetupDisabledBlackBox 按 ACCEPT-B9-03 验证 disabled 返回空 Providers。
func TestSetupDisabledBlackBox(t *testing.T) {
	p, err := telemetry.Setup(context.Background(), telemetry.Config{Enabled: false})
	if err != nil {
		t.Fatalf("disabled setup 不应报错: %v", err)
	}
	if p == nil {
		t.Fatalf("disabled 应返回空 Providers（Oracle: ACCEPT-B9-03）")
	}
	w, err := p.NewLogWriter(context.Background(), filepath.Join(t.TempDir(), "disabled.jsonl"))
	if err != nil {
		t.Fatalf("disabled 兼容 Runtime 应保留 file Writer: %v", err)
	}
	if _, ok := w.(*file.Writer); !ok {
		t.Fatalf("disabled Writer = %T, want *file.Writer", w)
	}
	if closer, ok := w.(interface{ Close(context.Context) error }); ok {
		_ = closer.Close(context.Background())
	}
}

// TestNewLogWriterEnvMatrixBlackBox 按 CASE-B9-04/05 验证出口决策矩阵：
// endpoint env 未设置 → *file.Writer（JSONL）；设置且启用 → *otlp.Writer。
func TestNewLogWriterEnvMatrixBlackBox(t *testing.T) {
	ctx := context.Background()
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	p, err := telemetry.Setup(ctx, telemetry.Config{Enabled: true, ServiceName: "svc"})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()

	w, err := p.NewLogWriter(ctx, filepath.Join(t.TempDir(), "blackbox.jsonl"))
	if err != nil {
		t.Fatalf("NewLogWriter(file) failed: %v", err)
	}
	if _, ok := w.(*file.Writer); !ok {
		t.Errorf("endpoint 未设置应返回 *file.Writer，实际 %T（Oracle: ACCEPT-B9-04）", w)
	}
	if c, ok := w.(interface{ Close(context.Context) error }); ok {
		_ = c.Close(ctx)
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	p2, err := telemetry.SetupFromEnvironment(ctx, telemetry.Config{ServiceName: "svc-otlp"})
	if err != nil {
		t.Fatalf("OTLP setup failed: %v", err)
	}
	defer func() { _ = p2.Shutdown(ctx) }()
	w2, err := p2.NewLogWriter(ctx, filepath.Join(t.TempDir(), "blackbox.jsonl"))
	if err != nil {
		t.Fatalf("NewLogWriter(otlp) failed: %v", err)
	}
	if _, ok := w2.(*otlp.Writer); !ok {
		t.Errorf("endpoint 设置应返回 *otlp.Writer，实际 %T（Oracle: ACCEPT-B9-05）", w2)
	}
	if c, ok := w2.(interface{ Close(context.Context) error }); ok {
		_ = c.Close(ctx)
	}
}

// TestNewLogWriterFreezesSetupDecision 验证 Writer 出口在 Setup 时固化，不受后续 env 变化影响。
func TestNewLogWriterFreezesSetupDecision(t *testing.T) {
	ctx := context.Background()
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	fileProviders, err := telemetry.SetupFromEnvironment(ctx, telemetry.Config{ServiceName: "file-svc"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fileProviders.Shutdown(ctx) }()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	w, err := fileProviders.NewLogWriter(ctx, filepath.Join(t.TempDir(), "frozen-file.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := w.(*file.Writer); !ok {
		t.Fatalf("Setup 后设置 env 得到 %T, want *file.Writer", w)
	}
	if c, ok := w.(interface{ Close(context.Context) error }); ok {
		_ = c.Close(ctx)
	}

	otlpProviders, err := telemetry.Setup(ctx, telemetry.Config{
		Enabled:     true,
		ServiceName: "otlp-svc",
		Endpoint:    "collector:4317",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = otlpProviders.Shutdown(ctx) }()
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	w, err = otlpProviders.NewLogWriter(ctx, filepath.Join(t.TempDir(), "frozen-otlp.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := w.(*otlp.Writer); !ok {
		t.Fatalf("显式 Config.Endpoint 得到 %T, want *otlp.Writer", w)
	}
	if c, ok := w.(interface{ Close(context.Context) error }); ok {
		_ = c.Close(ctx)
	}
}

// TestSetupNoEnvSideEffectBlackBox 按 CASE-B9-06 验证 R2：Setup 不改写 os.Environ。
func TestSetupNoEnvSideEffectBlackBox(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	ctx := context.Background()
	p, err := telemetry.Setup(ctx, telemetry.Config{Enabled: true, ServiceName: "svc"})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()
	if got := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); got != "collector:4317" {
		t.Errorf("Setup 不应改写 env，实际 %q（Oracle: ACCEPT-B9-05/RULE-B9-03）", got)
	}
}
