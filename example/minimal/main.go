// Command minimal 演示不依赖框架的 JSONL 写出。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/otelutil"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	ctx := context.Background()
	providers, err := telemetry.NewFileRuntime(telemetry.Config{
		ServiceName: "mall-monolith",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup file telemetry:", err)
		os.Exit(1)
	}
	restore := providers.InstallGlobal()
	defer restore()
	w, err := providers.NewWriter(ctx, telemetry.WriterConfig{FilePath: "logs/events.jsonl"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "create log writer:", err)
		os.Exit(1)
	}

	writeFailed := false
	logger := log.NewLogger(w,
		log.WithTraceExtractor(otelutil.NewTraceExtractor()),
		log.WithErrorHandler(func(_ context.Context, _ string, _ []slog.Attr, err error) {
			writeFailed = true
			fmt.Fprintln(os.Stderr, "write event:", err)
		}),
	)
	ctx, span := providers.Tracer("example/minimal").Start(ctx, "order.payment.succeeded")
	logger.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("order", "payment", "succeeded"),
			Result:    log.ResultSuccess,
		},
	})
	span.End()

	if closer, ok := w.(interface{ Close(context.Context) error }); ok {
		if err := closer.Close(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "close log writer:", err)
			os.Exit(1)
		}
	}
	if err := providers.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "shutdown telemetry:", err)
		os.Exit(1)
	}
	if writeFailed {
		os.Exit(1)
	}
}
