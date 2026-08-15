// Command grpc 演示 gRPC server span 与 rpc.server.duration 两个 unary 拦截器。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/10_grpc
//
// 教学要点：
//   - grpcmw.Trace 负责创建 server span 并注入 context；
//   - grpcmw.Metrics 负责记录 rpc.server.duration 直方图；
//   - 真实服务用 grpc.ChainUnaryInterceptor 一次挂载两个拦截器；
//   - 没有 proto 生成代码时，用 grpc.UnaryServerInfo 也能直接观察拦截器行为。
package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc"

	grpcmw "github.com/formal-you/go-observability/middleware/grpc"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	ctx := context.Background()

	// 默认本地演示：Trace 走 local（产生合法 TraceID/SpanID），Metric 关闭；
	// 设置 OTEL_EXPORTER_OTLP_ENDPOINT 后，Trace/Metric 统一改走 OTLP。
	traceOutput := telemetry.SignalOutputLocal
	metricOutput := telemetry.SignalOutputNone
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		traceOutput = telemetry.SignalOutputOTLP
		metricOutput = telemetry.SignalOutputOTLP
	}

	runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  telemetry.EnabledFromEnvironment(),
		Endpoint: telemetry.EndpointFromEnvironment(),
		Resource: telemetry.ResourceConfig{ServiceName: "grpc-demo", ServiceVersion: "0.1.0", Environment: "dev"},
		Trace:    telemetry.TraceConfig{Output: traceOutput},
		Metric:   telemetry.MetricConfig{Output: metricOutput},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputNone},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup telemetry:", err)
		os.Exit(1)
	}
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	// 真实服务装配：把两个拦截器按顺序挂到 gRPC Server。
	// 具体 service 由 protoc 生成代码通过 RegisterXxxServer 注册。
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcmw.Trace(grpcmw.TraceConfig{}),
		grpcmw.Metrics(grpcmw.MetricsConfig{}),
	))
	defer server.Stop()
	_ = server

	// 无 proto 时，直接以标准 UnaryServerInfo 驱动拦截器：
	// Trace 在外层，Metrics 在内层，最终 handler 返回 ok。
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Register"}
	traceInterceptor := grpcmw.Trace(grpcmw.TraceConfig{})
	metricsInterceptor := grpcmw.Metrics(grpcmw.MetricsConfig{})
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }

	if _, err := traceInterceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return metricsInterceptor(ctx, req, info, handler)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "interceptor:", err)
		os.Exit(1)
	}

	fmt.Println("OK: trace and metrics interceptors are configured; span/metric produced via Runtime providers")
}
