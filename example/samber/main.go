// Command samber 演示：把 go-observability 事件喂给 samber slog handler 链做对照实验。
// 借鉴验证：slog-multi（fanout/pipe）、slog-formatter（PII 掩码）、slog-sampling（采样）。
package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/samber/slog-formatter"
	"github.com/samber/slog-multi"
	"github.com/samber/slog-sampling"

	obs "github.com/formal-you/go-observability"
)

func main() {
	f, err := os.Create("samber-out.jsonl")
	if err != nil {
		slog.Error("create output", "err", err)
		os.Exit(1)
	}
	defer f.Close()

	// samber 链：采样 → PII 掩码 → fanout（JSON stdout + JSON 文件）
	chain := slogmulti.Pipe(
		slogsampling.UniformSamplingOption{Rate: 1.0}.NewMiddleware(), // 1.0=全量，便于对照；<1 即真实采样
		slogformatter.NewFormatterHandler(
			slogformatter.FormatByKey("mall.user_id", func(_ slog.Value) slog.Value {
				return slog.StringValue("***")
			}),
		),
	).Handler(
		slogmulti.Fanout(
			slog.NewJSONHandler(os.Stdout, nil),
			slog.NewJSONHandler(f, nil),
		),
	)
	logger := slog.New(chain)

	logger.Info("access", attrsToArgs(accessEvent().Attrs())...)
	logger.Info("business", attrsToArgs(businessEvent().Attrs())...)
}

func accessEvent() obs.AccessEvent {
	return obs.AccessEvent{
		EventMetadata: obs.EventMetadata{
			Timestamp: time.Now(),
			Level:     obs.LevelInfo,
			TraceID:   "0123456789abcdef0123456789abcdef",
			SpanID:    "0123456789abcdef",
			RequestID: "req-001",
			LatencyMS: 12,
		},
		Data: obs.AccessPayload{
			EventName: "access.http.request",
			Subject:   obs.Subject{UserID: "u_12345"},
			HTTP: obs.HTTPInfo{
				Method:     "GET",
				Route:      "/api/v1/products/:id",
				URLPath:    "/api/v1/products/42",
				StatusCode: 200,
				UserAgent:  "spike/1.0",
			},
			Result: obs.ResultSuccess,
		},
	}
}

func businessEvent() obs.BusinessEvent {
	return obs.BusinessEvent{
		EventMetadata: obs.EventMetadata{
			Timestamp: time.Now(),
			Level:     obs.LevelInfo,
			TraceID:   "0123456789abcdef0123456789abcdef",
			SpanID:    "0123456789abcdef",
			RequestID: "req-001",
		},
		Data: obs.BusinessPayload{
			EventName:    "business.order.paid",
			BusinessCode: "ORD-200",
			Result:       obs.ResultSuccess,
		},
	}
}

func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, 0, len(attrs))
	for _, a := range attrs {
		args = append(args, a)
	}
	return args
}
