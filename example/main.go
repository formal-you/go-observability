// Command example 演示 go-observability 的 Gin 中间件与日志写出。
// 默认把日志写入本目录 logs/events.jsonl；设置 OTEL_EXPORTER_OTLP_ENDPOINT 时改走 OTLP。
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/middleware/ginlog"
	"github.com/formal-you/go-observability/writer/file"
	"github.com/formal-you/go-observability/writer/otlp"
)

func main() {
	ctx := context.Background()
	setupTracer(ctx)
	w, err := newLogWriter(ctx)
	if err != nil {
		slog.Error("init log writer", "err", err)
		os.Exit(1)
	}
	defer closeWriter(ctx, w)
	logger := log.NewLogger(w)

	r := gin.New()
	r.Use(otelgin.Middleware("go-observability-demo"))
	r.Use(ginlog.Middleware(ginlog.Config{Logger: logger}))
	r.GET("/api/v1/products/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id")})
	})
	if err := r.Run(":8080"); err != nil {
		slog.Error("server exit", "err", err)
	}
}

// newLogWriter 优先 OTLP（OTEL_EXPORTER_OTLP_ENDPOINT），否则写入 example/logs/events.jsonl。
func newLogWriter(ctx context.Context) (log.Writer, error) {
	if ep := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); ep != "" {
		return otlp.New(ctx, otlp.WithEndpoint(ep))
	}
	return file.New(filepath.Join("logs", "events.jsonl"))
}

// closeWriter 关闭实现了 Close(ctx) 的 writer。
func closeWriter(ctx context.Context, w log.Writer) {
	if c, ok := w.(interface{ Close(context.Context) error }); ok {
		_ = c.Close(ctx)
	}
}

// setupTracer 配置全局 TracerProvider（otlptracegrpc），使 otelgin 生成真实 span。
func setupTracer(ctx context.Context) {
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		slog.Warn("trace exporter unavailable, use noop", "err", err)
		return
	}
	res := resource.NewWithAttributes(
		"https://opentelemetry.io/schemas/1.26.0",
		attribute.String("service.name", "go-observability"),
		attribute.String("service.version", "0.1.0"),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
}
