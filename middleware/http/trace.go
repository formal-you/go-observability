// httpmw：server span（net/http 适配）。
// 每个请求创建一个 server span，span context 注入 request context：下游 handler 与
// 日志事件（access/error）自动关联 trace_id/span_id。语义约定 1.41.0：
// span name = method + route（如 POST /api/v1/auth/register），属性
// http.request.method / http.response.status_code / http.route。
package httpmw

import (
	"net/http"

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
	// Route 返回 HTTP 路由模板写入 http.route；nil 时回退 net/http ServeMux 的 r.Pattern。
	Route func(r *http.Request) string
}

// Trace 返回为每个请求创建 server span 的 net/http 中间件。
func Trace(cfg TraceConfig) func(http.Handler) http.Handler {
	tracer := cfg.Tracer
	if tracer == nil {
		tracer = otel.Tracer("go-observability")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &mwutil.StatusRecorder{ResponseWriter: w}
			// 入口提取：读取上游 traceparent/tracestate，使本 span 接续调用方链路。
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, mwutil.HTTPSpanName(r.Method, r.URL.Path),
				trace.WithSpanKind(trace.SpanKindServer))
			defer mwutil.FinishSpan(span)
			r = r.WithContext(ctx)
			next.ServeHTTP(recorder, r)

			route := mwutil.HTTPRoute(cfg.Route, r)
			span.SetName(mwutil.HTTPSpanNameWithRoute(r.Method, route))
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.Int("http.response.status_code", recorder.Status),
			}
			if route != "" {
				attrs = append(attrs, attribute.String("http.route", route))
			}
			span.SetAttributes(attrs...)
			if recorder.Status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(recorder.Status))
			}
		})
	}
}
