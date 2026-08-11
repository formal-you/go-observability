// Command blackbox 通过真实 OTel span + Gin 请求生成语义化日志样例。
// 默认覆盖写入 sample.jsonl；-mode=otlp 时把同一批 trace/log 发送到本地 Collector。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/otelutil"
	"github.com/formal-you/go-observability/telemetry"
)

const blackboxServiceName = "go-observability-blackbox"

func main() {
	mode := flag.String("mode", "file", "输出模式：file 或 otlp")
	endpoint := flag.String("endpoint", "127.0.0.1:4317", "OTLP gRPC endpoint（仅 otlp 模式）")
	flag.Parse()

	if err := run(context.Background(), *mode, *endpoint); err != nil {
		fmt.Fprintln(os.Stderr, "blackbox:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, mode, endpoint string) error {
	switch mode {
	case "file":
		return runFileMode(ctx)
	case "otlp":
		return runOTLPMode(ctx, endpoint)
	default:
		return fmt.Errorf("不支持 mode=%q（可选 file/otlp）", mode)
	}
}

func runFileMode(ctx context.Context) error {
	path := filepath.Join("example", "blackbox", "sample.jsonl")
	report, err := writeFileSample(ctx, path)
	if err != nil {
		return err
	}
	fmt.Println("written:", path)
	printTraceReport(report)
	return nil
}

func writeFileSample(ctx context.Context, path string) (*scenarioReport, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("清理旧样例: %w", err)
	}
	providers, err := telemetry.SetupFile(telemetry.Config{
		ServiceName:    blackboxServiceName,
		ServiceVersion: "dev",
		Environment:    "local",
		Instance:       "blackbox-local",
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 file-only telemetry: %w", err)
	}
	w, err := providers.NewLogWriter(ctx, path)
	if err != nil {
		_ = providers.Shutdown(ctx)
		return nil, fmt.Errorf("创建 file writer: %w", err)
	}
	logger := log.NewLogger(w, log.WithTraceExtractor(otelutil.NewTraceExtractor()))
	report, emitErr := emitAll(ctx, logger, providers.Tracer(blackboxServiceName))
	closeErr := closeLogWriter(ctx, w)
	traceErr := providers.Shutdown(ctx)
	if err := errors.Join(emitErr, closeErr, traceErr); err != nil {
		return nil, err
	}
	return report, nil
}

func runOTLPMode(ctx context.Context, endpoint string) error {
	providers, err := telemetry.Setup(ctx, telemetry.Config{
		Enabled:           true,
		ServiceName:       blackboxServiceName,
		ServiceVersion:    "dev",
		Environment:       "local",
		Endpoint:          endpoint,
		TraceSampleRatio:  1,
		TraceBatchTimeout: 100 * time.Millisecond,
		LogBatchTimeout:   100 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("初始化 telemetry: %w", err)
	}
	w, err := providers.NewLogWriter(ctx, filepath.Join("example", "blackbox", "sample.jsonl"))
	if err != nil {
		_ = providers.Shutdown(ctx)
		return fmt.Errorf("创建 OTLP writer: %w", err)
	}
	logger := log.NewLogger(w, log.WithTraceExtractor(otelutil.NewTraceExtractor()))
	report, emitErr := emitAll(ctx, logger, providers.Tracer(blackboxServiceName))
	closeErr := closeLogWriter(ctx, w)
	shutdownErr := providers.Shutdown(ctx)
	if err := errors.Join(emitErr, closeErr, shutdownErr); err != nil {
		return err
	}
	fmt.Println("exported OTLP:", endpoint)
	printTraceReport(report)
	return nil
}

func closeLogWriter(ctx context.Context, writer log.Writer) error {
	if closer, ok := writer.(interface{ Close(context.Context) error }); ok {
		return closer.Close(ctx)
	}
	return nil
}

func printTraceReport(report *scenarioReport) {
	traces := report.snapshot()
	names := make([]string, 0, len(traces))
	for name := range traces {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("trace %-24s %s\n", name, traces[name].TraceID)
	}
}
