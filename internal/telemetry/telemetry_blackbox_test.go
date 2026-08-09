// Package telemetry_test 是 internal/telemetry 的外部黑盒测试层（package telemetry_test）。
// 期望值来自 observability-design/spec/acceptance.md 的 B9 验收契约（Oracle），只用公开 API，
// 不读取实现内部——防"同代理写实现+测试"的自证幻觉。用例引用 CASE/RULE/ACCEPT ID。
package telemetry_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/formal-you/go-observability/internal/telemetry"
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
	if p == nil || p.Resource() != nil || p.LoggerProvider() != nil {
		t.Fatalf("disabled 应返回空 Providers（Oracle: ACCEPT-B9-03）")
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
	w2, err := p.NewLogWriter(ctx, filepath.Join(t.TempDir(), "blackbox.jsonl"))
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
