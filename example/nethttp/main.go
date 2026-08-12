// Command nethttp 演示不使用 Gin 时的完整链路：
// 库中间件 trace（server span）/ metrics（http.server.request.duration）/
// nethttp（显式错误收口 + panic 恢复）自动装配；access 事件由接入方 10 行包装
// （库暂无 net/http access 中间件，Gin 版见 ginlog；示例模板见下方 accessLog）。
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/formal-you/go-observability/errs"
	log "github.com/formal-you/go-observability/log"
	httpmw "github.com/formal-you/go-observability/middleware/http"
	"github.com/formal-you/go-observability/middleware/otelutil"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	ctx := context.Background()

	endpoint := telemetry.EndpointFromEnvironment()
	output := telemetry.LogOutputFile
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		output = telemetry.LogOutputOTLP
	}
	providers, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled: telemetry.EnabledFromEnvironment(), ServiceName: "nethttp-demo", ServiceVersion: "0.1.0", Environment: "dev",
		Endpoint: endpoint, LogOutput: output,
	})
	if err != nil {
		slog.Error("init telemetry", "err", err)
		os.Exit(1)
	}
	restore := providers.InstallGlobal()
	defer restore()
	defer func() { _ = providers.Shutdown(ctx) }()

	writerCfg := telemetry.WriterConfig{FilePath: "logs/nethttp-events.jsonl"}
	if output == telemetry.LogOutputOTLP {
		writerCfg = telemetry.WriterConfig{}
	}
	w, err := providers.NewWriter(ctx, writerCfg)
	if err != nil {
		slog.Error("init log writer", "err", err)
		os.Exit(1)
	}
	defer closeWriter(ctx, w)
	logger := log.NewLogger(w,
		log.WithTraceExtractor(otelutil.NewTraceExtractor()),
		log.WithMasker(log.FieldMasker{}),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/v1/orders", func(rw http.ResponseWriter, _ *http.Request) {
		// 显式错误统一收口：SetError 挂载，nethttp.ErrorResponse 决定状态码/响应体并写错误事件。
		httpmw.SetError(rw, mustBusinessError(errs.BusinessErrorConfig{
			Code: "ORDER.CREATE.STOCK_INSUFFICIENT", Type: "business.stock_insufficient", Message: "库存不足",
		}))
	})

	// 链顺序：trace（server span）→ recover（panic 收口）→ accessLog（接入方模板）
	// → metrics（http.server.request.duration）→ errorResponse（显式错误收口）。
	var handler http.Handler = mux
	handler = httpmw.Trace(httpmw.TraceConfig{})(handler)
	handler = httpmw.Recover(httpmw.ErrorConfig{Logger: logger})(handler)
	handler = accessLog(logger)(handler)
	handler = httpmw.Metrics(httpmw.MetricsConfig{})(handler)
	handler = httpmw.ErrorResponse(httpmw.ErrorConfig{Logger: logger})(handler)

	slog.Info("listen :8081")
	if err := http.ListenAndServe(":8081", handler); err != nil {
		slog.Error("server", "err", err)
	}
}

func mustBusinessError(cfg errs.BusinessErrorConfig) errs.BizError {
	err, buildErr := errs.NewBusinessError(cfg)
	if buildErr != nil {
		panic(buildErr)
	}
	return err
}

// accessLog 是 net/http access 事件的接入方模板（库暂无 net/http access 中间件）。
func accessLog(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			// 心跳/健康检查是确定性噪音：直接短路，不产生 access 事件（不靠概率采样）。
			if r.URL.Path == "/healthz" {
				next.ServeHTTP(rw, r)
				return
			}
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: rw}
			next.ServeHTTP(recorder, r)
			status := recorder.status
			level := log.LevelInfo
			if status >= 500 && status != 503 {
				level = log.LevelError
			} else if status >= 400 {
				level = log.LevelWarn
			}
			result := log.ResultSuccess
			if status >= 400 {
				result = log.ResultFailed
			}
			logger.Emit(r.Context(), log.AccessEvent{
				EventMetadata: log.EventMetadata{
					Level:     level,
					LatencyMS: time.Since(start).Milliseconds(),
				},
				Data: log.AccessPayload{
					EventName: log.EventNameHTTPRequestCompleted,
					HTTP: log.HTTPInfo{
						Method:     r.Method,
						URLPath:    r.URL.Path,
						StatusCode: status,
						ClientIP:   clientIP(r),
						UserAgent:  r.UserAgent(),
					},
					Result: result,
				},
			})
		})
	}
}

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

func clientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

func closeWriter(ctx context.Context, w log.ManagedWriter) {
	_ = w.Close(ctx)
}
