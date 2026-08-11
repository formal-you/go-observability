// Package grpcmw 提供 gRPC 框架体系的拦截器：server span 与请求指标。
// 其他框架体系见 middleware/gin（Gin）与 middleware/http（net/http）。
package grpcmw

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/formal-you/go-observability/middleware/internal/mwutil"
	"github.com/formal-you/go-observability/middleware/otelutil"
)

// TraceConfig 配置 server span 拦截器。
type TraceConfig struct {
	// Tracer 用于创建 span；nil 时使用全局 otel.Tracer("go-observability")。
	Tracer trace.Tracer
}

// Trace 返回为每个 RPC 调用创建 server span 的 gRPC unary 拦截器。
// 语义约定 1.41.0：span name = FullMethod，属性 rpc.system / rpc.service /
// rpc.method / rpc.grpc.status_code。
func Trace(cfg TraceConfig) grpc.UnaryServerInterceptor {
	tracer := cfg.Tracer
	if tracer == nil {
		tracer = otel.Tracer("go-observability")
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		// 入口提取：读取 gRPC metadata 的 traceparent/tracestate，使本 span 接续调用方链路。
		ctx = otel.GetTextMapPropagator().Extract(ctx, otelutil.IncomingMetadata(ctx))
		ctx, span := tracer.Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer mwutil.FinishSpan(span)
		resp, err = handler(ctx, req)

		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", mwutil.RPCService(info.FullMethod)),
			attribute.String("rpc.method", mwutil.RPCMethod(info.FullMethod)),
			attribute.Int("rpc.grpc.status_code", int(status.Code(err))),
		)
		if err != nil && status.Code(err) != grpccodes.OK {
			span.SetStatus(codes.Error, status.Code(err).String())
		}
		return resp, err
	}
}
