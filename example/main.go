// Command example 演示 go-observability 的三信号装配（telemetry）+ Gin 全链路中间件
// （ginmw.Trace / Recover / AccessLog / Metrics / ErrorResponse）。
// 默认把日志写入当前工作目录的 logs/events.jsonl；设置 OTEL_EXPORTER_OTLP_ENDPOINT 时改走 OTLP。
// 设 OTEL_SDK_DISABLED=true 可离线运行（trace/metric/log provider 全部 noop）。
package main

import (
	"context"
	"errors"
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
	output := telemetry.LogOutputFile
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		output = telemetry.LogOutputOTLP
	}
	// 三信号装配：Runtime 构造与全局安装显式分开。
	providers, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:        telemetry.EnabledFromEnvironment(),
		ServiceName:    "go-observability",
		ServiceVersion: "0.1.0",
		Environment:    "dev",
		Region:         os.Getenv("GO_OBSERVABILITY_REGION"),
		Instance:       os.Getenv("GO_OBSERVABILITY_INSTANCE"),
		Endpoint:       endpoint,
		LogOutput:      output,
	})
	if err != nil {
		slog.Error("init telemetry", "err", err)
		os.Exit(1)
	}
	restore := providers.InstallGlobal()
	defer restore()
	defer func() { _ = providers.Shutdown(ctx) }()

	w, err := newLogWriter(ctx, providers)
	if err != nil {
		slog.Error("init log writer", "err", err)
		os.Exit(1)
	}
	defer closeWriter(ctx, w)
	logger := log.NewLogger(w)

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
		ginmw.ErrorResponse(ginmw.ErrorConfig{Logger: logger}),
	)
	r.GET("/api/v1/products/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id")})
	})
	r.POST("/api/v1/orders", func(c *gin.Context) {
		// 业务拒绝：errresp.Abort 挂载错误，收口中间件决定状态码/响应体并写错误事件。
		ginmw.Abort(c, mustBusinessError(errs.BusinessErrorConfig{
			Code: "ORDER.CREATE.STOCK_INSUFFICIENT", Type: "business.stock_insufficient", Message: "库存不足",
		}))
	})
	if err := r.Run(":8080"); err != nil {
		slog.Error("server exit", "err", err)
	}
}

func mustBusinessError(cfg errs.BusinessErrorConfig) errs.BizError {
	err, buildErr := errs.NewBusinessError(cfg)
	if buildErr != nil {
		panic(buildErr)
	}
	return err
}

// newLogWriter 优先 OTLP（OTEL_EXPORTER_OTLP_ENDPOINT），否则写入当前工作目录的 logs/events.jsonl。
// OTLP 路径注入 telemetry 的 Resource 与 LoggerProvider，三信号共享同一份资源与装配。
func newLogWriter(ctx context.Context, p *telemetry.Providers) (log.Writer, error) {
	if p == nil {
		return nil, errors.New("nil telemetry runtime")
	}
	if p.LoggerProvider() != nil {
		return p.NewWriter(ctx, telemetry.WriterConfig{})
	}
	return p.NewWriter(ctx, telemetry.WriterConfig{FilePath: filepath.Join("logs", "events.jsonl")})
}

// closeWriter 关闭实现了 Close(ctx) 的 writer。
func closeWriter(ctx context.Context, w log.Writer) {
	if c, ok := w.(interface{ Close(context.Context) error }); ok {
		_ = c.Close(ctx)
	}
}
