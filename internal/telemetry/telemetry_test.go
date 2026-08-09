package telemetry

import (
	"context"
	"os"
	"testing"
)

func TestSetupDisabled(t *testing.T) {
	ctx := context.Background()
	p, err := Setup(ctx, Config{Enabled: false})
	if err != nil {
		t.Fatalf("disabled setup 不应报错: %v", err)
	}
	if p == nil || p.Resource() != nil || p.LoggerProvider() != nil {
		t.Fatalf("disabled setup 应返回空 Providers，got %+v", p)
	}
	if p.Tracer("x") == nil {
		t.Error("disabled setup 的 Tracer 应回退全局（非 nil）")
	}
	if p.Meter("x") == nil {
		t.Error("disabled setup 的 Meter 应回退全局（非 nil）")
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("disabled setup 的 Shutdown 应返回 nil, got %v", err)
	}
}

func TestSetupRequiresServiceName(t *testing.T) {
	_, err := Setup(context.Background(), Config{Enabled: true})
	if err == nil {
		t.Fatal("缺少 service.name 应报错")
	}
}

func TestSetupInvalidSampleRatio(t *testing.T) {
	for _, ratio := range []float64{-0.1, 1.5} {
		if _, err := Setup(context.Background(), Config{Enabled: true, ServiceName: "svc", TraceSampleRatio: ratio}); err == nil {
			t.Errorf("ratio=%v 应报错", ratio)
		}
	}
}

func TestSetupResourceAttributes(t *testing.T) {
	ctx := context.Background()
	p, err := Setup(ctx, Config{
		Enabled:        true,
		ServiceName:    "svc",
		ServiceVersion: "1.2.3",
		Environment:    "test",
		Region:         "cn",
		Instance:       "i-1",
		Endpoint:       "127.0.0.1:4317",
	})
	if err != nil {
		t.Fatalf("setup 失败: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()
	if p.Resource() == nil || p.LoggerProvider() == nil {
		t.Fatal("enabled setup 应返回非 nil 的 Resource 与 LoggerProvider")
	}

	want := map[string]string{
		"service.name":           "svc",
		"service.version":        "1.2.3",
		"deployment.environment": "test",
		"region":                 "cn",
		"instance":               "i-1",
	}
	got := map[string]string{}
	for _, kv := range p.Resource().Attributes() {
		got[string(kv.Key)] = kv.Value.Emit()
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("resource 属性 %s = %q, want %q（全部: %v）", k, got[k], v, got)
		}
	}
}

func TestShutdownNilAndDisabled(t *testing.T) {
	ctx := context.Background()
	if err := (*Providers)(nil).Shutdown(ctx); err != nil {
		t.Errorf("nil Shutdown 应返回 nil, got %v", err)
	}
	if err := (&Providers{}).Shutdown(ctx); err != nil {
		t.Errorf("空 Providers Shutdown 应返回 nil, got %v", err)
	}
}

func TestEnabledFromEnvironment(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	if EnabledFromEnvironment() {
		t.Error("OTEL_SDK_DISABLED=true 时应禁用")
	}
	t.Setenv("OTEL_SDK_DISABLED", "")
	if !EnabledFromEnvironment() {
		t.Error("未设置时应默认启用")
	}
}

func TestEndpointFromEnvironment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	if got := EndpointFromEnvironment(); got != "collector:4317" {
		t.Errorf("env endpoint = %q", got)
	}
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if got := EndpointFromEnvironment(); got != defaultEndpoint {
		t.Errorf("缺省 endpoint = %q, want %q", got, defaultEndpoint)
	}
}

func TestEndpointURL(t *testing.T) {
	if got := endpointURL("127.0.0.1:4317"); got != "http://127.0.0.1:4317" {
		t.Errorf("host:port 规范化 = %q", got)
	}
	if got := endpointURL("https://collector:4317"); got != "https://collector:4317" {
		t.Errorf("完整 URL 不应改写 = %q", got)
	}
}
