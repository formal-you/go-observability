// Command otel_logs 演示同一事件在 file 与 OTLP 两种出口下的投影。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/14_otel_logs
//	Get-Content .\logs\otel-logs.jsonl
//
// 设置 OTEL_EXPORTER_OTLP_ENDPOINT 后，同一份业务埋点改走 OTLP，代码无需重写：
//
//	$env:OTEL_EXPORTER_OTLP_ENDPOINT = "127.0.0.1:4317"
//	go run ./example/14_otel_logs
//
// 教学要点：file 输出扁平 JSONL，OTLP 输出把 timestamp / level / trace/span
// 映射到 LogRecord 顶层字段。完整映射见 docs/reference/otel-logs-data-model.md。
package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "otel_logs:", err)
		os.Exit(1)
	}
}

// run 根据环境变量选择 Log 出口，演示业务埋点不随出口变化而重写。
func run(ctx context.Context) error {
	output := telemetry.SignalOutputFile
	filePath := "logs/otel-logs.jsonl"
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		output = telemetry.SignalOutputOTLP
		filePath = ""
	}

	runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  telemetry.EnabledFromEnvironment(),
		Endpoint: telemetry.EndpointFromEnvironment(),
		Resource: telemetry.ResourceConfig{ServiceName: "otel-logs-demo", ServiceVersion: "0.1.0", Environment: "dev"},
		Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputLocal},
		Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputNone},
		Log:      telemetry.LogConfig{Output: output, FilePath: filePath},
	})
	if err != nil {
		return err
	}
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	w, err := runtime.NewLogWriter(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = w.Close(ctx) }()

	logger := log.NewLogger(w)

	// 手动创建 span，让 file JSONL 中的 trace_id/span_id 与 OTLP 顶层字段有同源对照。
	spanCtx, span := runtime.Tracer("example/otel_logs").Start(ctx, "order.payment.succeeded")
	logger.Emit(spanCtx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("order", "payment", "succeeded"),
			Result:    log.ResultSuccess,
		},
	})
	span.End()

	if output == telemetry.SignalOutputFile {
		fmt.Println("written: logs/otel-logs.jsonl")
	} else {
		fmt.Println("exported OTLP logs to:", telemetry.EndpointFromEnvironment())
	}
	return nil
}
