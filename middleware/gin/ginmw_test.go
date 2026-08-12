package ginmw

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// captureWriter 捕获 Logger 写出的 eventType 与扁平 attrs，用于断言中间件发出的事件形状。
type captureWriter struct {
	eventTypes []string
	attrsList  [][]slog.Attr
}

func (w *captureWriter) Write(_ context.Context, eventType string, attrs ...slog.Attr) error {
	w.eventTypes = append(w.eventTypes, eventType)
	w.attrsList = append(w.attrsList, attrs)
	return nil
}

func attrMap(attrs []slog.Attr) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

func attrString(t *testing.T, attrs map[string]any, key, want string) {
	t.Helper()
	v, ok := attrs[key].(slog.Value)
	if !ok {
		t.Errorf("缺少属性 %s（实际: %v）", key, keysOf(attrs))
		return
	}
	if got := v.String(); got != want {
		t.Errorf("%s = %v, want %s", key, got, want)
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func doRequest(r *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// captureProcessor 收集 OnEnd 的 span 供断言。
type captureProcessor struct {
	spans []sdktrace.ReadOnlySpan
}

func (p *captureProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}
func (p *captureProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	p.spans = append(p.spans, s)
}
func (p *captureProcessor) Shutdown(context.Context) error   { return nil }
func (p *captureProcessor) ForceFlush(context.Context) error { return nil }

func newTestTracer(t *testing.T) (trace.Tracer, *captureProcessor) {
	t.Helper()
	p := &captureProcessor{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(p), sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	// 全局 TextMapPropagator 默认 no-op；模拟生产（telemetry.Setup 安装 W3C TraceContext）。
	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() { otel.SetTextMapPropagator(prevProp) })

	return tp.Tracer("go-observability-test"), p
}

func attrStr(t *testing.T, span sdktrace.ReadOnlySpan, key string) string {
	t.Helper()
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	t.Fatalf("span 缺少属性 %s（实际: %v）", key, span.Attributes())
	return ""
}

func attrInt(t *testing.T, span sdktrace.ReadOnlySpan, key string) int64 {
	t.Helper()
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsInt64()
		}
	}
	t.Fatalf("span 缺少属性 %s（实际: %v）", key, span.Attributes())
	return 0
}

// newTestMeter 创建 ManualReader 驱动的 MeterProvider 并设为全局，测试结束恢复。
func newTestMeter(t *testing.T) (metric.Meter, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(prev) })
	return otel.Meter("go-observability-test"), reader
}

// collectHistogram 从 ManualReader 收集指定名称的直方图数据点。
func collectHistogram(t *testing.T, reader *sdkmetric.ManualReader, name string) []metricdata.HistogramDataPoint[float64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("metric %s data = %T, want Histogram[float64]", name, m.Data)
			}
			return hist.DataPoints
		}
	}
	t.Fatalf("metric %s not found", name)
	return nil
}

func metricAttrInt(t *testing.T, attrs attribute.Set, key string) int64 {
	t.Helper()
	v, ok := attrs.Value(attribute.Key(key))
	if !ok {
		t.Fatalf("attributes 缺少 %s: %v", key, attrs)
	}
	return v.AsInt64()
}

func metricAttrString(t *testing.T, attrs attribute.Set, key string) string {
	t.Helper()
	v, ok := attrs.Value(attribute.Key(key))
	if !ok {
		t.Fatalf("attributes 缺少 %s: %v", key, attrs)
	}
	return v.AsString()
}
