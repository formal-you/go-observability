// Command example 演示 go-observability 的三信号装配（telemetry）+ Gin 全链路中间件
// （ginmw.Trace / Recover / AccessLog / Metrics / ErrorResponse）。
// 默认把日志写入当前工作目录的 logs/events.jsonl，并把 Trace 设为 local、Metric 设为 none；
// 设置 OTEL_EXPORTER_OTLP_ENDPOINT 时三信号统一改走 OTLP。
// 设 OTEL_SDK_DISABLED=true 可离线运行（trace/metric/log provider 全部 noop，日志仍写本地文件）。
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
	ginmw "github.com/formal-you/go-observability/middleware/gin"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	ctx := context.Background()

	endpoint := telemetry.EndpointFromEnvironment()
	logOutput := telemetry.SignalOutputFile
	traceOutput := telemetry.SignalOutputLocal
	metricOutput := telemetry.SignalOutputNone
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		logOutput = telemetry.SignalOutputOTLP
		traceOutput = telemetry.SignalOutputOTLP
		metricOutput = telemetry.SignalOutputOTLP
	}
	// 三信号装配：Runtime 构造与全局安装显式分开。
	runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  telemetry.EnabledFromEnvironment(),
		Endpoint: endpoint,
		Resource: telemetry.ResourceConfig{
			ServiceName:    "go-observability",
			ServiceVersion: "0.1.0",
			Environment:    "dev",
			Region:         os.Getenv("GO_OBSERVABILITY_REGION"),
			Instance:       os.Getenv("GO_OBSERVABILITY_INSTANCE"),
		},
		Trace:  telemetry.TraceConfig{Output: traceOutput},
		Metric: telemetry.MetricConfig{Output: metricOutput},
		Log:    telemetry.LogConfig{Output: logOutput, FilePath: filepath.Join("logs", "events.jsonl")},
	})
	if err != nil {
		slog.Error("init telemetry", "err", err)
		os.Exit(1)
	}
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	w, err := runtime.NewLogWriter(ctx)
	if err != nil {
		slog.Error("init log writer", "err", err)
		os.Exit(1)
	}
	defer closeWriter(ctx, w)
	logger := log.NewLogger(w)

	stockInsufficientErr, err := errs.NewBusinessError(errs.BusinessErrorConfig{
		Code: "ORDER.CREATE.STOCK_INSUFFICIENT", Type: "FAILED_PRECONDITION", Message: "库存不足",
	})
	if err != nil {
		slog.Error("build business error", "err", err)
		os.Exit(1)
	}

	// 全链路中间件（注册顺序即执行顺序）：
	// trace（server span，注入 ctx）→ ginlog（access 事件）→ recover（panic 收口）
	// → metrics（http.server.request.duration）→ errresp（显式错误收口）。
	// AccessLog 必须包在 Recover 外层，才能在 panic 被收口后记录最终 500 响应。
	r := gin.New()
	r.Use(
		ginmw.Trace(ginmw.TraceConfig{}),
		ginmw.AccessLog(ginmw.AccessConfig{Logger: logger}),
		ginmw.Recover(ginmw.RecoverConfig{Logger: logger}),
		ginmw.Metrics(ginmw.MetricsConfig{}),
		ginmw.ErrorResponse(ginmw.ErrorConfig{Logger: logger, EventName: log.NewEventName("order", "create", "stock_insufficient")}),
	)
	r.GET("/api/v1/products/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id")})
	})
	r.POST("/api/v1/orders", func(c *gin.Context) {
		// 业务拒绝：errresp.Abort 挂载错误，收口中间件决定状态码/响应体并写错误事件。
		ginmw.Abort(c, stockInsufficientErr)
	})
	if err := r.Run(":8080"); err != nil {
		slog.Error("server exit", "err", err)
	}
}

func closeWriter(ctx context.Context, w log.ManagedWriter) {
	_ = w.Close(ctx)
}
