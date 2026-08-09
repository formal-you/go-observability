// Command minimal 演示不依赖框架的 JSONL 写出。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	log "github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/writer/file"
)

func main() {
	ctx := context.Background()
	w, err := file.New("logs/events.jsonl")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create log writer:", err)
		os.Exit(1)
	}

	writeFailed := false
	logger := log.NewLogger(w, log.WithErrorHandler(func(_ context.Context, _ string, _ []slog.Attr, err error) {
		writeFailed = true
		fmt.Fprintln(os.Stderr, "write event:", err)
	}))
	logger.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("business", "order", "paid"),
			Result:    log.ResultSuccess,
		},
	})

	if err := w.Close(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "close log writer:", err)
		os.Exit(1)
	}
	if writeFailed {
		os.Exit(1)
	}
}
