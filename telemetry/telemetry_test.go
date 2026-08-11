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
		"service.name":                "svc",
		"service.version":             "1.2.3",
		"deployment.environment.name": "test",
		"region":                      "cn",
		"service.instance.id":         "i-1",
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

func TestSetupFileCreatesLocalTracesAndMetadata(t *testing.T) {
	ctx := context.Background()
	p, err := SetupFile(Config{ServiceName: "mall-monolith", ServiceVersion: "1.0.0", Instance: "shop-01"})
	if err != nil {
		t.Fatalf("SetupFile 失败: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()
	if p.LoggerProvider() != nil {
		t.Fatal("SetupFile 不应创建 OTLP LoggerProvider")
	}
	attrs := map[string]string{}
	for _, kv := range p.Resource().Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	if attrs["service.name"] != "mall-monolith" || attrs["service.version"] != "1.0.0" || attrs["service.instance.id"] != "shop-01" || attrs["deployment.environment.name"] != "development" {
		t.Fatalf("SetupFile Resource = %v", attrs)
	}
	tracer := p.Tracer("test")
	_, span1 := tracer.Start(ctx, "one")
	_, span2 := tracer.Start(ctx, "two")
	if !span1.SpanContext().TraceID().IsValid() || !span1.SpanContext().SpanID().IsValid() {
		t.Fatal("本地 span1 ID 无效")
	}
	if span1.SpanContext().IsSampled() {
		t.Fatal("file-only root span 不应设置 sampled 标记")
	}
	if !span2.SpanContext().TraceID().IsValid() || span1.SpanContext().TraceID() == span2.SpanContext().TraceID() {
		t.Fatal("独立本地 root span 应有不同有效 TraceID")
	}
	span1.End()
	span2.End()
}

func TestSetupFileRequiresServiceName(t *testing.T) {
	if _, err := SetupFile(Config{}); err == nil {
		t.Fatal("SetupFile 缺少 service.name 应报错")
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
	got, err := endpointURL("127.0.0.1:4317")
	if err != nil || got != "http://127.0.0.1:4317" {
		t.Errorf("host:port 规范化 = %q", got)
	}
	got, err = endpointURL("https://collector:4317")
	if err != nil || got != "https://collector:4317" {
		t.Errorf("完整 URL 不应改写 = %q", got)
	}
	for _, invalid := range []string{
		"collector",
		"collector:",
		":4317",
		"collector:not-a-port",
		"collector:0",
		"collector:70000",
		"://bad",
		"ftp://collector:4317",
		"https://user@collector:4317",
		"https://collector:4317/v1/logs",
	} {
		if _, err := endpointURL(invalid); err == nil {
			t.Errorf("endpointURL(%q) 应返回错误", invalid)
		}
	}
}

func TestSetupRejectsNegativeDurations(t *testing.T) {
	tests := []Config{
		{Enabled: true, ServiceName: "svc", TraceBatchTimeout: -1},
		{Enabled: true, ServiceName: "svc", MetricExportInterval: -1},
		{Enabled: true, ServiceName: "svc", LogBatchTimeout: -1},
	}
	for _, cfg := range tests {
		if _, err := Setup(context.Background(), cfg); err == nil {
			t.Errorf("Setup(%+v) 应拒绝负数周期", cfg)
		}
	}
}

func TestSetupRejectsInvalidEndpoint(t *testing.T) {
	for _, endpoint := range []string{"collector", ":4317", "collector:not-a-port", "grpc://collector:4317"} {
		_, err := Setup(context.Background(), Config{
			Enabled:     true,
			ServiceName: "svc",
			Endpoint:    endpoint,
		})
		if err == nil {
			t.Errorf("Setup Endpoint=%q error = nil, want non-nil", endpoint)
		}
	}
}
