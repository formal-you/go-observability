// Package trace 提供基于全局 TracerProvider 的 HTTP/gRPC 服务器链路中间件。
// 每个请求创建一个 server span，span context 注入 request context：下游 handler 与
// 日志事件（access/error）自动关联 trace_id/span_id。语义约定 1.41.0：
//   - HTTP：span name = method + route（如 POST /api/v1/auth/register），属性
//     http.request.method / http.response.status_code / http.route；
//   - gRPC：span name = FullMethod，属性 rpc.system / rpc.service / rpc.method /
//     rpc.grpc.status_code。
//
// 默认使用全局 TracerProvider（telemetry.Setup 已全局安装），可经 Config.Tracer 注入。
// handler panic 时本中间件标记 span 为错误后重抛，由外层 recover 中间件收口。
package trace

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config 中间件配置。
type Config struct {
	// Tracer 用于创建 span；nil 时使用全局 otel.Tracer("go-observability")。
	Tracer trace.Tracer
	// Route 返回 HTTP 路由模板写入 http.route；nil 时 net/http 回退 r.Pattern。
	Route func(r *http.Request) string
}

// defaultTracer 返回显式注入或全局 go-observability Tracer。
func defaultTracer(cfg Config) trace.Tracer {
	if cfg.Tracer != nil {
		return cfg.Tracer
	}
	return otel.Tracer("go-observability")
}

// NewHTTPMiddleware 返回为每个请求创建 server span 的 net/http 中间件。
func NewHTTPMiddleware(cfg Config) func(http.Handler) http.Handler {
	tracer := defaultTracer(cfg)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w}
			ctx, span := tracer.Start(r.Context(), httpSpanName(r.Method, r.URL.Path),
				trace.WithSpanKind(trace.SpanKindServer))
			defer finishSpan(span)
			r = r.WithContext(ctx)
			next.ServeHTTP(recorder, r)

			route := httpRoute(cfg, r)
			span.SetName(httpSpanNameWithRoute(r.Method, route))
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.Int("http.response.status_code", recorder.status),
			}
			if route != "" {
				attrs = append(attrs, attribute.String("http.route", route))
			}
			span.SetAttributes(attrs...)
			if recorder.status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(recorder.status))
			}
		})
	}
}

// NewGinMiddleware 返回为每个请求创建 server span 的 Gin 中间件（http.route 取路由模板）。
func NewGinMiddleware(cfg Config) gin.HandlerFunc {
	tracer := defaultTracer(cfg)
	return func(c *gin.Context) {
		route := c.FullPath()
		ctx, span := tracer.Start(c.Request.Context(), httpSpanName(c.Request.Method, route),
			trace.WithSpanKind(trace.SpanKindServer))
		defer finishSpan(span)
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

// NewGRPCUnaryInterceptor 返回为每个 RPC 调用创建 server span 的 gRPC unary 拦截器。
func NewGRPCUnaryInterceptor(cfg Config) grpc.UnaryServerInterceptor {
	tracer := defaultTracer(cfg)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		ctx, span := tracer.Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer finishSpan(span)
		resp, err = handler(ctx, req)

		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", rpcService(info.FullMethod)),
			attribute.String("rpc.method", rpcMethod(info.FullMethod)),
			attribute.Int("rpc.grpc.status_code", int(status.Code(err))),
		)
		if err != nil && status.Code(err) != grpccodes.OK {
			span.SetStatus(codes.Error, status.Code(err).String())
		}
		return resp, err
	}
}

// finishSpan 结束 span；handler panic 时标记错误后重抛，由外层 recover 中间件收口。
func finishSpan(span trace.Span) {
	if recovered := recover(); recovered != nil {
		span.SetStatus(codes.Error, "panic")
		span.End()
		panic(recovered)
	}
	span.End()
}

// httpSpanName 生成初始 span name：method + target（gin 的 route 或 net/http 的 path）。
func httpSpanName(method, target string) string {
	if target == "" {
		return method
	}
	return method + " " + target
}

// httpSpanNameWithRoute 在路由已知后重命名 span：ServeMux 的 r.Pattern 自带 method
// （如 "GET /ok"），避免重复拼装。
func httpSpanNameWithRoute(method, route string) string {
	if route == "" {
		return method
	}
	if strings.HasPrefix(route, method) {
		return route
	}
	return method + " " + route
}

// httpRoute 返回路由模板；Config.Route 为空时回退 net/http ServeMux 的 r.Pattern。
func httpRoute(cfg Config, r *http.Request) string {
	if cfg.Route != nil {
		return cfg.Route(r)
	}
	return r.Pattern
}

// statusRecorder 记录响应状态码（net/http 默认 200，WriteHeader 覆盖）。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(data)
}

func rpcService(fullMethod string) string {
	parts := strings.Split(fullMethod, "/")
	if len(parts) >= 3 {
		return parts[1]
	}
	return ""
}

func rpcMethod(fullMethod string) string {
	parts := strings.Split(fullMethod, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}
