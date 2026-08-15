// Command metrics 演示使用方如何用 go-observability 的 MeterProvider 自建指标。
//
// 服务端 RED 中间件（自动记录 http.server.request.duration）见 middleware/metrics；
// 本文件演示使用方自定义指标（业务 counter）模板。
// 跑法：
//
//	OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317 go run ./example/13_metrics
//
// 需本地 observability 栈（Mimir）已起；仅验证编译可无 endpoint（provider 仍会尝试连默认地址）。
// 离线：OTEL_SDK_DISABLED=true go run ./example/13_metrics
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	ctx := context.Background()
	p, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  telemetry.EnabledFromEnvironment(),
		Endpoint: telemetry.EndpointFromEnvironment(),
		Resource: telemetry.ResourceConfig{ServiceName: "metrics-demo", ServiceVersion: "0.1.0", Environment: "dev"},
		Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputNone},
		Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputOTLP},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputNone},
	})
	if err != nil {
		slog.Error("telemetry setup", "err", err)
		os.Exit(1)
	}
	restore := p.InstallGlobal()
	defer restore()
	defer func() { _ = p.Shutdown(ctx) }()

	// 使用方命名空间：任意 instrument 名；与库事件模型无关。
	meter := p.Meter("example/shop")

	// RED：请求耗时直方图（秒）。桶由使用方按 SLA 选择；此处用 OTel 默认桶。
	reqDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("HTTP server request duration (consumer-defined)"),
	)
	if err != nil {
		slog.Error("histogram", "err", err)
		os.Exit(1)
	}

	// 业务 counter：订单支付成功次数（示例语义，非库契约）。
	ordersPaid, err := meter.Int64Counter(
		"order.payment.succeeded",
		metric.WithDescription("orders paid successfully (consumer-defined)"),
	)
	if err != nil {
		slog.Error("counter", "err", err)
		os.Exit(1)
	}

	// 模拟一次请求 + 一次业务成功。
	reqDuration.Record(ctx, 0.042, metric.WithAttributes(
		attribute.String("http.request.method", "POST"),
		attribute.String("http.route", "/api/v1/orders/{id}/pay"),
		attribute.Int("http.response.status_code", 200),
	))
	ordersPaid.Add(ctx, 1, metric.WithAttributes(
		attribute.String("pay_channel", "wechat"), // 低基数；禁止 user_id/order_id
	))

	// PeriodicReader 默认 15s 才导出；短 demo 主动等一轮便于在 Mimir 看到点。
	if os.Getenv("OTEL_SDK_DISABLED") != "true" {
		slog.Info("metrics recorded; waiting 16s for export interval")
		time.Sleep(16 * time.Second)
	}
	slog.Info("done",
		"hint_promql_p99", `histogram_quantile(0.99, sum(rate(http_server_request_duration_seconds_bucket[5m])) by (le, http_route))`,
		"hint_promql_paid", `sum(rate(business_order_paid_total[5m])) by (pay_channel)`,
	)
}
