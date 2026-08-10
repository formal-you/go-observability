package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func attrInt(t *testing.T, attrs attribute.Set, key string) int64 {
	t.Helper()
	v, ok := attrs.Value(attribute.Key(key))
	if !ok {
		t.Fatalf("attributes 缺少 %s: %v", key, attrs)
	}
	return v.AsInt64()
}

func attrString(t *testing.T, attrs attribute.Set, key string) string {
	t.Helper()
	v, ok := attrs.Value(attribute.Key(key))
	if !ok {
		t.Fatalf("attributes 缺少 %s: %v", key, attrs)
	}
	return v.AsString()
}

func TestHTTPMiddlewareRecordsDuration(t *testing.T) {
	_, reader := newTestMeter(t)
	handler := NewHTTPMiddleware(Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	points := collectHistogram(t, reader, metricHTTPServerRequestDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if attrString(t, points[0].Attributes, "http.request.method") != "GET" {
		t.Fatalf("method attr = %s, want GET", attrString(t, points[0].Attributes, "http.request.method"))
	}
	if attrInt(t, points[0].Attributes, "http.response.status_code") != http.StatusOK {
		t.Fatalf("status attr = %d, want 200", attrInt(t, points[0].Attributes, "http.response.status_code"))
	}
	if points[0].Count != 1 {
		t.Fatalf("count = %d, want 1", points[0].Count)
	}
}

func TestHTTPMiddlewareRouteFromMuxPattern(t *testing.T) {
	_, reader := newTestMeter(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := NewHTTPMiddleware(Config{})(mux)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))

	points := collectHistogram(t, reader, metricHTTPServerRequestDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if got := attrString(t, points[0].Attributes, "http.route"); got != "GET /ok" {
		t.Fatalf("http.route = %q, want GET /ok（ServeMux r.Pattern）", got)
	}
}

func TestGinMiddlewareRecordsDuration(t *testing.T) {
	_, reader := newTestMeter(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(NewGinMiddleware(Config{}))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))

	points := collectHistogram(t, reader, metricHTTPServerRequestDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if attrString(t, points[0].Attributes, "http.route") != "/ok" {
		t.Fatalf("http.route = %q, want /ok", attrString(t, points[0].Attributes, "http.route"))
	}
	if attrInt(t, points[0].Attributes, "http.response.status_code") != http.StatusOK {
		t.Fatalf("status attr = %d, want 200", attrInt(t, points[0].Attributes, "http.response.status_code"))
	}
}

func TestGRPCUnaryInterceptorRecordsDuration(t *testing.T) {
	_, reader := newTestMeter(t)
	interceptor := NewGRPCUnaryInterceptor(Config{})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Register"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}

	points := collectHistogram(t, reader, metricRPCServerDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if attrString(t, points[0].Attributes, "rpc.service") != "mall.auth.v1.AuthService" {
		t.Fatalf("rpc.service = %q", attrString(t, points[0].Attributes, "rpc.service"))
	}
	if attrString(t, points[0].Attributes, "rpc.method") != "Register" {
		t.Fatalf("rpc.method = %q", attrString(t, points[0].Attributes, "rpc.method"))
	}
	if attrInt(t, points[0].Attributes, "rpc.grpc.status_code") != int64(codes.OK) {
		t.Fatalf("status attr = %d, want OK", attrInt(t, points[0].Attributes, "rpc.grpc.status_code"))
	}
}

func TestGRPCUnaryInterceptorErrorStatus(t *testing.T) {
	_, reader := newTestMeter(t)
	interceptor := NewGRPCUnaryInterceptor(Config{})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Login"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.InvalidArgument, "bad")
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
	points := collectHistogram(t, reader, metricRPCServerDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if attrInt(t, points[0].Attributes, "rpc.grpc.status_code") != int64(codes.InvalidArgument) {
		t.Fatalf("status attr = %d, want InvalidArgument", attrInt(t, points[0].Attributes, "rpc.grpc.status_code"))
	}
}
