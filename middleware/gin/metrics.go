// ginmw：请求指标（Gin 适配）。
// 指标命名与属性对齐 Semantic Conventions 1.41.0：`http.server.request.duration`
// 直方图（秒），属性 http.request.method / http.response.status_code / http.route。
package ginmw

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/formal-you/go-observability/middleware/internal/mwutil"
)

const metricHTTPServerRequestDuration = "http.server.request.duration"

// defaultMeter 返回显式注入或全局 go-observability Meter。
func defaultMeter(cfg MetricsConfig) metric.Meter {
	if cfg.Meter != nil {
		return cfg.Meter
	}
	return otel.Meter("go-observability")
}

// MetricsConfig 配置请求指标中间件。
type MetricsConfig struct {
	// Meter 指标 Meter；nil 时使用全局 otel.Meter("go-observability")。
	Meter metric.Meter
}

// Metrics 返回记录 http.server.request.duration 的 Gin 中间件（http.route 取路由模板）。
func Metrics(cfg MetricsConfig) gin.HandlerFunc {
	histogram := mwutil.Histogram(defaultMeter(cfg), metricHTTPServerRequestDuration,
		"Duration of HTTP server requests.", "s")
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", c.Request.Method),
			attribute.Int("http.response.status_code", c.Writer.Status()),
		}
		if route := c.FullPath(); route != "" {
			attrs = append(attrs, attribute.String("http.route", route))
		}
		histogram.Record(c.Request.Context(), time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	}
}
