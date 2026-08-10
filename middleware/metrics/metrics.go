// Package metrics 提供基于全局 Meter 的 HTTP/gRPC 服务器指标中间件。
// 指标命名与属性对齐 Semantic Conventions 1.41.0：
//   - HTTP：`http.server.request.duration` 直方图（秒），属性 http.request.method /
//     http.response.status_code / http.route（net/http 用 r.Pattern，Gin 用 c.FullPath()）；
//   - gRPC：`rpc.server.duration` 直方图（秒），属性 rpc.system / rpc.service /
//     rpc.method / rpc.grpc.status_code。
//
// 默认使用 go.opentelemetry.io/otel 全局 MeterProvider（telemetry.Setup 已全局安装），
// 也可经 Config.Meter 显式注入；采样/导出频率由 telemetry 的 PeriodicReader 控制。
package metrics

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Config 中间件配置。
type Config struct {
	// Meter 指标 Meter；nil 时使用全局 otel.Meter("go-observability")。
	Meter metric.Meter
	// Route 返回 HTTP 路由模板写入 http.route；nil 时 net/http 回退 r.Pattern。
	Route func(r *http.Request) string
}

// defaultMeter 返回显式注入或全局 go-observability Meter。
func defaultMeter(cfg Config) metric.Meter {
	if cfg.Meter != nil {
		return cfg.Meter
	}
	return otel.Meter("go-observability")
}

const (
	metricHTTPServerRequestDuration = "http.server.request.duration"
	metricRPCServerDuration         = "rpc.server.duration"
)

// NewHTTPMiddleware 返回记录 http.server.request.duration 的 net/http 中间件。
func NewHTTPMiddleware(cfg Config) func(http.Handler) http.Handler {
	histogram := histogram(cfg, metricHTTPServerRequestDuration,
		"Duration of HTTP server requests.", "s")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.Int("http.response.status_code", recorder.status),
			}
			if route := httpRoute(cfg, r); route != "" {
				attrs = append(attrs, attribute.String("http.route", route))
			}
			histogram.Record(r.Context(), time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		})
	}
}

// NewGinMiddleware 返回记录 http.server.request.duration 的 Gin 中间件（http.route 取路由模板）。
func NewGinMiddleware(cfg Config) gin.HandlerFunc {
	histogram := histogram(cfg, metricHTTPServerRequestDuration,
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

// NewGRPCUnaryInterceptor 返回记录 rpc.server.duration 的 gRPC unary 拦截器。
func NewGRPCUnaryInterceptor(cfg Config) grpc.UnaryServerInterceptor {
	histogram := histogram(cfg, metricRPCServerDuration,
		"Duration of RPC server requests.", "s")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)
		attrs := []attribute.KeyValue{
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", rpcService(info.FullMethod)),
			attribute.String("rpc.method", rpcMethod(info.FullMethod)),
			attribute.Int("rpc.grpc.status_code", int(status.Code(err))),
		}
		histogram.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		return resp, err
	}
}

// histogram 创建 Float64Histogram；创建失败即 panic（配置错误应尽早暴露）。
func histogram(cfg Config, name, description, unit string) metric.Float64Histogram {
	h, err := defaultMeter(cfg).Float64Histogram(name,
		metric.WithDescription(description), metric.WithUnit(unit))
	if err != nil {
		panic("metrics: create histogram " + name + ": " + err.Error())
	}
	return h
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
