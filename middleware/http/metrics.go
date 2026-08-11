// httpmw：请求指标（net/http 适配）。
// 指标命名与属性对齐 Semantic Conventions 1.41.0：`http.server.request.duration`
// 直方图（秒），属性 http.request.method / http.response.status_code / http.route。
package httpmw

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/formal-you/go-observability/middleware/internal/mwutil"
)

const metricHTTPServerRequestDuration = "http.server.request.duration"

// MetricsConfig 配置请求指标中间件。
type MetricsConfig struct {
	// Meter 指标 Meter；nil 时使用全局 otel.Meter("go-observability")。
	Meter metric.Meter
	// Route 返回 HTTP 路由模板写入 http.route；nil 时回退 net/http ServeMux 的 r.Pattern。
	Route func(r *http.Request) string
}

// defaultMeter 返回显式注入或全局 go-observability Meter。
func defaultMeter(cfg MetricsConfig) metric.Meter {
	if cfg.Meter != nil {
		return cfg.Meter
	}
	return otel.Meter("go-observability")
}

// Metrics 返回记录 http.server.request.duration 的 net/http 中间件。
func Metrics(cfg MetricsConfig) func(http.Handler) http.Handler {
	histogram := mwutil.Histogram(defaultMeter(cfg), metricHTTPServerRequestDuration,
		"Duration of HTTP server requests.", "s")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &mwutil.StatusRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.Int("http.response.status_code", recorder.Status),
			}
			if route := mwutil.HTTPRoute(cfg.Route, r); route != "" {
				attrs = append(attrs, attribute.String("http.route", route))
			}
			histogram.Record(r.Context(), time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		})
	}
}
