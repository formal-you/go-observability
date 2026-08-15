// Command telemetry 演示四种 Runtime 预设，以及它们如何映射环境变量。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/07_telemetry -mode=file
//	go run ./example/07_telemetry -mode=log
//	go run ./example/07_telemetry -mode=all-file
//	go run ./example/07_telemetry -mode=otlp -endpoint=127.0.0.1:4317
//
// 教学要点：
//   - NewFileRuntime / NewLogRuntime / NewOTLPRuntime / NewAllFileRuntime
//     是覆盖常见场景的便捷入口；
//   - 需要自定义采样、批量导出间隔、队列或轮转时，改用 NewRuntime 分信号配置；
//   - 所有预设都返回同一个 Runtime 所有权边界：Writer、Provider 与 Shutdown。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	mode := flag.String("mode", "file", "预设：file / log / all-file / otlp")
	endpoint := flag.String("endpoint", "", "OTLP gRPC endpoint（仅 otlp 模式）")
	flag.Parse()

	if err := run(context.Background(), *mode, *endpoint); err != nil {
		fmt.Fprintln(os.Stderr, "telemetry:", err)
		os.Exit(1)
	}
}

// run 根据 mode 选择预设并完成一次“构造 -> 写事件 -> 关闭”的完整生命周期。
func run(ctx context.Context, mode, endpoint string) error {
	switch mode {
	case "file":
		// NewFileRuntime 等价于 Trace=local、Metric=none、Log=file：
		// 小单体无需 Collector，仍能产生合法 TraceID/SpanID 并写本地 JSONL。
		runtime, err := telemetry.NewFileRuntime(telemetry.Config{
			Resource: telemetry.ResourceConfig{ServiceName: "telemetry-demo", ServiceVersion: "0.1.0", Environment: "dev"},
			Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/telemetry-events.jsonl"},
		})
		if err != nil {
			return err
		}
		if err := emitAndClose(ctx, runtime); err != nil {
			return err
		}
		fmt.Println("file preset written: logs/telemetry-events.jsonl")
		return nil

	case "log":
		// NewLogRuntime 只装配 Log 信号；Trace/Metric 保持进程级 no-op。
		runtime, err := telemetry.NewLogRuntime(ctx, "telemetry-demo", telemetry.SignalOutputFile, "logs/telemetry-log.jsonl")
		if err != nil {
			return err
		}
		if err := emitAndClose(ctx, runtime); err != nil {
			return err
		}
		fmt.Println("log preset written: logs/telemetry-log.jsonl")
		return nil

	case "all-file":
		// NewAllFileRuntime 把 Trace/Metric/Log 三信号都写到指定目录，
		// 适合完全离线的本地排查或教学演示。
		runtime, err := telemetry.NewAllFileRuntime(ctx, "telemetry-demo", "logs/telemetry-all")
		if err != nil {
			return err
		}
		if err := emitAndClose(ctx, runtime); err != nil {
			return err
		}
		fmt.Println("all-file preset written: logs/telemetry-all/{events,trace,metric}.jsonl")
		return nil

	case "otlp":
		// NewOTLPRuntime 把三信号统一发往 Collector；没有 Collector 时
		// 后台导出失败由 OTel SDK 诊断日志报告，不会由 Emit 返回给调用方。
		if endpoint == "" {
			return fmt.Errorf("otlp 模式需要 -endpoint（例如 127.0.0.1:4317）")
		}
		runtime, err := telemetry.NewOTLPRuntime(ctx, "telemetry-demo", endpoint)
		if err != nil {
			return err
		}
		if err := emitAndClose(ctx, runtime); err != nil {
			return err
		}
		fmt.Println("otlp preset exported to:", endpoint)
		return nil

	default:
		return fmt.Errorf("未知 mode=%q，可选 file/log/all-file/otlp", mode)
	}
}

// emitAndClose 演示预设共有的资源所有权模式：
// InstallGlobal -> NewWriter -> Emit -> Close Writer -> Shutdown Runtime。
func emitAndClose(ctx context.Context, runtime *telemetry.Runtime) error {
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	w, err := runtime.NewWriter(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = w.Close(ctx) }()

	logger := log.NewLogger(w)
	logger.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("order", "payment", "succeeded"),
			Result:    log.ResultSuccess,
		},
	})
	return nil
}
