// Command example 演示 go-observability 的三信号装配（telemetry）+ Gin 全链路中间件
// （trace / recover / ginlog / metrics / errresp）。
// 默认把日志写入当前工作目录的 logs/events.jsonl；设置 OTEL_EXPORTER_OTLP_ENDPOINT 时改走 OTLP。
// 设 OTEL_SDK_DISABLED=true 可离线运行（trace/metric/log provider 全部 noop）。
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/middleware/errresp"
	"github.com/formal-you/go-observability/middleware/ginlog"
	"github.com/formal-you/go-observability/middleware/metrics"
	recovermw "github.com/formal-you/go-observability/middleware/recover"
	tracemw "github.com/formal-you/go-observability/middleware/trace"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	ctx := context.Background()

	// 三信号装配：全局安装 trace、metric、log provider。
	providers, err := telemetry.SetupFromEnvironment(ctx, telemetry.Config{
		ServiceName:    "go-observability",
		ServiceVersion: "0.1.0",
		Environment:    "dev",
		Region:         os.Getenv("GO_OBSERVABILITY_REGION"),
		Instance:       os.Getenv("GO_OBSERVABILITY_INSTANCE"),
	})
	if err != nil {
		slog.Error("init telemetry", "err", err)
		os.Exit(1)
	}
	defer func() { _ = providers.Shutdown(ctx) }()

	w, err := newLogWriter(ctx, providers)
	if err != nil {
		slog.Error("init log writer", "err", err)
		os.Exit(1)
	}
	defer closeWriter(ctx, w)
	logger := log.NewLogger(w)

	// 全链路中间件（注册顺序即执行顺序）：
	// trace（server span，注入 ctx）→ recover（panic 收口）→ ginlog（access 事件）
	// → metrics（http.server.request.duration）→ errresp（显式错误收口）。
	r := gin.New()
	r.Use(
		tracemw.NewGinMiddleware(tracemw.Config{}),
		recovermw.Middleware(recovermw.Config{Logger: logger}),
		ginlog.Middleware(ginlog.Config{Logger: logger}),
		metrics.NewGinMiddleware(metrics.Config{}),
		errresp.Middleware(errresp.Config{Logger: logger}),
	)
	r.GET("/api/v1/products/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id")})
	})
	r.POST("/api/v1/orders", func(c *gin.Context) {
		// 业务拒绝：errresp.Abort 挂载错误，收口中间件决定状态码/响应体并写错误事件。
		errresp.Abort(c, errs.NewBusiness("ORDER.CREATE.STOCK_INSUFFICIENT", "business.order.stock_insufficient", "库存不足"))
	})
	if err := r.Run(":8080"); err != nil {
		slog.Error("server exit", "err", err)
	}
}

// newLogWriter 优先 OTLP（OTEL_EXPORTER_OTLP_ENDPOINT），否则写入当前工作目录的 logs/events.jsonl。
// OTLP 路径注入 telemetry 的 Resource 与 LoggerProvider，三信号共享同一份资源与装配。
func newLogWriter(ctx context.Context, p *telemetry.Providers) (log.Writer, error) {
	// 出口决策由 SetupFromEnvironment 固化，NewLogWriter 复用该决策。
	return p.NewLogWriter(ctx, filepath.Join("logs", "events.jsonl"))
}

// closeWriter 关闭实现了 Close(ctx) 的 writer。
func closeWriter(ctx context.Context, w log.Writer) {
	if c, ok := w.(interface{ Close(context.Context) error }); ok {
		_ = c.Close(ctx)
	}
}
