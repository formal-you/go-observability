// grpcmw：请求指标（gRPC 适配）。
// 指标命名与属性对齐 Semantic Conventions 1.41.0：`rpc.server.duration` 直方图（秒），
// 属性 rpc.system / rpc.service / rpc.method / rpc.grpc.status_code。
package grpcmw

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/formal-you/go-observability/middleware/internal/mwutil"
)

const metricRPCServerDuration = "rpc.server.duration"

// MetricsConfig 配置请求指标拦截器。
type MetricsConfig struct {
	// Meter 指标 Meter；nil 时使用全局 otel.Meter("go-observability")。
	Meter metric.Meter
}

// defaultMeter 返回显式注入或全局 go-observability Meter。
func defaultMeter(cfg MetricsConfig) metric.Meter {
	if cfg.Meter != nil {
		return cfg.Meter
	}
	return otel.Meter("go-observability")
}

// Metrics 返回记录 rpc.server.duration 的 gRPC unary 拦截器。
func Metrics(cfg MetricsConfig) grpc.UnaryServerInterceptor {
	histogram := mwutil.Histogram(defaultMeter(cfg), metricRPCServerDuration,
		"Duration of RPC server requests.", "s")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)
		attrs := []attribute.KeyValue{
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", mwutil.RPCService(info.FullMethod)),
			attribute.String("rpc.method", mwutil.RPCMethod(info.FullMethod)),
			attribute.Int("rpc.grpc.status_code", int(status.Code(err))),
		}
		histogram.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		return resp, err
	}
}
