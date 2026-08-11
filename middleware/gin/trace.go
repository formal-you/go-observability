// ginmw：server span（Gin 适配）。
// 每个请求创建一个 server span，span context 注入 request context：下游 handler 与
// 日志事件（access/error）自动关联 trace_id/span_id。语义约定 1.41.0：
// span name = method + route（如 POST /api/v1/auth/register），属性
// http.request.method / http.response.status_code / http.route。
package ginmw

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/middleware/internal/mwutil"
)

// TraceConfig 配置 server span 中间件。
type TraceConfig struct {
	// Tracer 用于创建 span；nil 时使用全局 otel.Tracer("go-observability")。
	Tracer trace.Tracer
	// Route 返回 HTTP 路由模板写入 http.route；nil 时 net/http 回退 r.Pattern（gin 用 c.FullPath）。
	Route func(r *http.Request) string
}

// Trace 返回为每个请求创建 server span 的 Gin 中间件（http.route 取路由模板）。
func Trace(cfg TraceConfig) gin.HandlerFunc {
	tracer := cfg.Tracer
	if tracer == nil {
		tracer = otel.Tracer("go-observability")
	}
	return func(c *gin.Context) {
		route := c.FullPath()
		// 入口提取：读取上游 traceparent/tracestate，使本 span 接续调用方链路。
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := tracer.Start(ctx, mwutil.HTTPSpanName(c.Request.Method, route),
			trace.WithSpanKind(trace.SpanKindServer))
		defer mwutil.FinishSpan(span)
		c.Request = c.Request.WithContext(ctx)
		c.Next()

		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", c.Request.Method),
			attribute.Int("http.response.status_code", c.Writer.Status()),
		}
		if route != "" {
			attrs = append(attrs, attribute.String("http.route", route))
		}
		span.SetAttributes(attrs...)
		if c.Writer.Status() >= 500 {
			span.SetStatus(codes.Error, http.StatusText(c.Writer.Status()))
		}
	}
}
