// Command quickstart 是“最小可运行”教学示例：不依赖任何 Web 框架，
// 展示一条 BusinessEvent 从构造、trace 关联到写入本地 JSONL 的完整链路。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/01_quickstart
//	Get-Content .\logs\events.jsonl
//
// 预期输出：logs/events.jsonl 新增一条 type=business 的扁平 JSONL。
// 本文件刻意只保留一条事件，帮助读者先把 Runtime、Writer、Logger 三者关系看清楚。
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

	// NewFileRuntime 是 file-only 预设：Trace=local、Metric=none、Log=file。
	// 它适合无需 Collector 的小单体：不依赖 OTLP 网络，仍能产生合法 TraceID/SpanID
	// 并把服务身份写入每条 JSONL。
	providers, err := telemetry.NewFileRuntime(telemetry.Config{
		Resource: telemetry.ResourceConfig{ServiceName: "mall-monolith"},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/events.jsonl"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup file telemetry:", err)
		os.Exit(1)
	}

	// Runtime 构造不会修改进程级 OTel global state；需要让依赖全局 API 的组件
	// 使用这些 Provider 时，调用 InstallGlobal，并在退出时调用 restore 恢复现场。
	restore := providers.InstallGlobal()
	defer restore()

	// NewWriter 创建的 ManagedWriter 拥有与 Runtime 一致的 Resource 身份；
	// 关闭顺序应是先 Close Writer（释放 Writer 自己拥有的资源），再 Shutdown
	// Runtime（按 Log -> Metric -> Trace 关闭 Provider）。
	w, err := providers.NewWriter(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create log writer:", err)
		os.Exit(1)
	}

	// WithTraceExtractor 让 Logger 在事件未显式携带 trace_id/span_id 时，
	// 从 context 的 span context 自动补全；WithErrorHandler 则观察 Writer
	// 写入失败，错误不会反向传播给业务调用方。
	writeFailed := false
	logger := log.NewLogger(w,
		log.WithTraceExtractor(otelutil.NewTraceExtractor()),
		log.WithErrorHandler(func(_ context.Context, _ string, _ []slog.Attr, err error) {
			writeFailed = true
			fmt.Fprintln(os.Stderr, "write event:", err)
		}),
	)

	// 手动创建 span，让读者看清 trace_id/span_id 的来源；真实 Web 服务通常由
	// middleware/gin.Trace 或 middleware/http.Trace 自动注入，不需要手工 Start。
	ctx, span := providers.Tracer("example/01_quickstart").Start(ctx, "order.payment.succeeded")
	logger.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("order", "payment", "succeeded"),
			Result:    log.ResultSuccess,
		},
	})
	span.End()

	// 资源关闭顺序：先 Writer，再 Runtime。
	if err := w.Close(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "close log writer:", err)
		os.Exit(1)
	}
	if err := providers.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "shutdown telemetry:", err)
		os.Exit(1)
	}
	if writeFailed {
		os.Exit(1)
	}
}
