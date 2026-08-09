// Command nethttp 演示无 Gin 时使用方如何记 access 事件（C3）。
// 框架中间件仅官方提供 ginlog/recover；net/http 由接入方 10 行包装即可。
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	log "github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/writer/file"
)

func main() {
	w, err := file.New("logs/nethttp-events.jsonl")
	if err != nil {
		slog.Error("writer", "err", err)
		os.Exit(1)
	}
	defer func() { _ = w.Close(context.Background()) }()

	logger := log.NewLogger(w,
		log.WithSampler(log.ResultKeepSampler{Ratio: 1}),
		log.WithMasker(log.FieldMasker{}),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(rw http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("ok"))
		emitAccess(logger, r, http.StatusOK, time.Since(start))
	})

	slog.Info("listen :8081")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		slog.Error("server", "err", err)
	}
}

func emitAccess(logger *log.Logger, r *http.Request, status int, d time.Duration) {
	var ip net.IP
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = net.ParseIP(host)
	}
	result := log.ResultSuccess
	if status >= 400 {
		result = log.ResultFailed
	}
	level := log.LevelInfo
	if status >= 500 && status != 503 {
		level = log.LevelError
	} else if status >= 400 {
		level = log.LevelWarn
	}
	logger.Emit(r.Context(), log.AccessEvent{
		EventMetadata: log.EventMetadata{
			Level:     level,
			LatencyMS: d.Milliseconds(),
		},
		Data: log.AccessPayload{
			EventName: log.EventNameAccessHTTPRequest,
			HTTP: log.HTTPInfo{
				Method:     r.Method,
				URLPath:    r.URL.Path,
				StatusCode: status,
				ClientIP:   ip,
				UserAgent:  r.UserAgent(),
			},
			Result: result,
		},
	})
}
