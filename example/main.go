// Command example 演示 go-observability 的三信号装配（telemetry）+ Gin 中间件 + 日志写出。
// 默认把日志写入当前工作目录的 logs/events.jsonl；设置 OTEL_EXPORTER_OTLP_ENDPOINT 时改走 OTLP。
// 设 OTEL_SDK_DISABLED=true 可离线运行（trace/metric/log provider 全部 noop）。
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/middleware/ginlog"
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

	// metric 由使用方自建；完整 RED 和业务 counter 见 example/metrics。
	httpDuration, err := providers.Meter("go-observability/example").Int64Histogram(
		"http.server.request.duration",
		metric.WithUnit("ms"),
		metric.WithDescription("HTTP server request duration (consumer-defined)"),
	)
	if err != nil {
		slog.Warn("init histogram", "err", err)
	}

	r := gin.New()
	r.Use(otelgin.Middleware("go-observability-demo"))
	r.Use(ginlog.Middleware(ginlog.Config{Logger: logger}))
	r.GET("/api/v1/products/:id", func(c *gin.Context) {
		start := time.Now()
		c.JSON(200, gin.H{"id": c.Param("id")})
		if httpDuration != nil {
			httpDuration.Record(c.Request.Context(), time.Since(start).Milliseconds(),
				metric.WithAttributes(
					attribute.String("http.request.method", "GET"),
					attribute.String("http.route", c.FullPath()),
					attribute.Int("http.response.status_code", 200),
				))
		}
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
