// Command errorhandler 演示 WithErrorHandler：观察 Writer 写入失败。
// 用一个「前 2 次成功、之后全部失败」的内存 Writer，展示写入失败时回调被同步触发；
// 同时验证失败不会作为业务返回值向上传播，也不中断后续事件写出。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	log "github.com/formal-you/go-observability/log"
)

// flakyWriter 前 maxOK 次写入成功，之后全部失败。
type flakyWriter struct {
	written int
	maxOK   int
}

func (w *flakyWriter) Write(_ context.Context, msg string, _ ...slog.Attr) error {
	w.written++
	if w.written > w.maxOK {
		return fmt.Errorf("flaky writer failed on event #%d (%s)", w.written, msg)
	}
	return nil
}

func main() {
	ctx := context.Background()

	var observed []error
	logger := log.NewLogger(&flakyWriter{maxOK: 2},
		log.WithErrorHandler(func(_ context.Context, msg string, _ []slog.Attr, err error) {
			observed = append(observed, err)
			fmt.Fprintln(os.Stderr, "error handler observed: msg =", msg, "| err =", err)
		}),
	)

	// 写 4 个事件：第 1、2 个成功，第 3、4 个触发 ErrorHandler。
	for i := 1; i <= 4; i++ {
		logger.Emit(ctx, log.BusinessEvent{
			EventMetadata: log.EventMetadata{Level: log.LevelInfo},
			Data: log.BusinessPayload{
				EventName: log.NewEventName("order", "payment", "succeeded"),
				Result:    log.ResultSuccess,
			},
		})
		fmt.Printf("emitted event #%d\n", i)
	}

	if len(observed) != 2 {
		fmt.Fprintf(os.Stderr, "want 2 write failures observed, got %d\n", len(observed))
		os.Exit(1)
	}
	fmt.Println("OK: WithErrorHandler observed 2 write failures; pipeline continued and errors were not propagated to callers")
}
