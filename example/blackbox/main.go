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
	"github.com/formal-you/go-observability/logwriter/file"
	"github.com/formal-you/go-observability/middleware/otelutil"
	"github.com/formal-you/go-observability/telemetry"
)

const blackboxServiceName = "go-observability-blackbox"

func main() {
	mode := flag.String("mode", "file", "输出模式：file 或 otlp")
	configPath := flag.String("config", filepath.Join("example", "blackbox", "config.example.yaml"), "应用 YAML 配置文件路径")
	endpoint := flag.String("endpoint", "", "覆盖配置中的 OTLP gRPC endpoint（仅 otlp 模式）")
	flag.Parse()

	if err := run(context.Background(), *mode, *endpoint, *configPath); err != nil {
		fmt.Fprintln(os.Stderr, "blackbox:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, mode, endpoint, configPath string) error {
	cfg, err := loadBlackboxConfig(configPath)
	if err != nil {
		return err
	}
	switch mode {
	case "file":
		return runFileMode(ctx, cfg)
	case "otlp":
		if endpoint == "" {
			endpoint = cfg.OTLP.Endpoint
		}
		if endpoint == "" {
			return errors.New("OTLP 模式要求配置 otlp.endpoint 或传入 -endpoint")
		}
		return runOTLPMode(ctx, endpoint, cfg)
	default:
		return fmt.Errorf("不支持 mode=%q（可选 file/otlp）", mode)
	}
}

func runFileMode(ctx context.Context, cfg blackboxConfig) error {
	report, err := writeConfiguredFileSample(ctx, cfg)
	if err != nil {
		return err
	}
	fmt.Println("written:", cfg.Logs.OutputPath)
	printTraceReport(report)
	return nil
}

func writeFileSample(ctx context.Context, path string) (*scenarioReport, error) {
	cfg := defaultBlackboxConfig()
	cfg.Logs.OutputPath = path
	return writeConfiguredFileSample(ctx, cfg)
}

func writeConfiguredFileSample(ctx context.Context, cfg blackboxConfig) (*scenarioReport, error) {
	if cfg.Logs.OverwriteOnStart {
		if err := os.Remove(cfg.Logs.OutputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("清理旧样例: %w", err)
		}
	}
	level, err := configuredLevel(cfg.Logs.Level)
	if err != nil {
		return nil, err
	}
	providers, err := telemetry.NewFileRuntime(telemetry.Config{
		Resource: telemetry.ResourceConfig{
			ServiceName:    cfg.Service.Name,
			ServiceVersion: cfg.Service.Version,
			Environment:    cfg.Service.Environment,
			Instance:       cfg.Service.InstanceID,
		},
		Log: telemetry.LogConfig{
			Output:      telemetry.SignalOutputFile,
			FilePath:    cfg.Logs.OutputPath,
			FileOptions: fileOptsFor(cfg),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 file-only telemetry: %w", err)
	}
	restore := providers.InstallGlobal()
	w, err := providers.NewLogWriter(ctx)
	if err != nil {
		restore()
		_ = providers.Shutdown(ctx)
		return nil, fmt.Errorf("创建 file writer: %w", err)
	}
	logger := log.NewLogger(w,
		log.WithMinLevel(level),
		log.WithTraceExtractor(otelutil.NewTraceExtractor()),
	)
	report, emitErr := emitAll(ctx, logger, providers.Tracer(cfg.Service.Name))
	closeErr := closeLogWriter(ctx, w)
	restore()
	traceErr := providers.Shutdown(ctx)
	if err := errors.Join(emitErr, closeErr, traceErr); err != nil {
		return nil, err
	}
	return report, nil
}

func runOTLPMode(ctx context.Context, endpoint string, cfg blackboxConfig) error {
	level, err := configuredLevel(cfg.Logs.Level)
	if err != nil {
		return err
	}
	providers, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  true,
		Endpoint: endpoint,
		Resource: telemetry.ResourceConfig{
			ServiceName:    cfg.Service.Name,
			ServiceVersion: cfg.Service.Version,
			Environment:    cfg.Service.Environment,
			Instance:       cfg.Service.InstanceID,
		},
		Trace: telemetry.TraceConfig{
			Output:       telemetry.SignalOutputOTLP,
			SampleRatio:  1,
			BatchTimeout: 100 * time.Millisecond,
		},
		Metric: telemetry.MetricConfig{Output: telemetry.SignalOutputOTLP},
		Log: telemetry.LogConfig{
			Output:       telemetry.SignalOutputOTLP,
			BatchTimeout: 100 * time.Millisecond,
		},
	})
	if err != nil {
		return fmt.Errorf("初始化 telemetry: %w", err)
	}
	restore := providers.InstallGlobal()
	w, err := providers.NewLogWriter(ctx)
	if err != nil {
		restore()
		_ = providers.Shutdown(ctx)
		return fmt.Errorf("创建 OTLP writer: %w", err)
	}
	logger := log.NewLogger(w,
		log.WithMinLevel(level),
		log.WithTraceExtractor(otelutil.NewTraceExtractor()),
	)
	report, emitErr := emitAll(ctx, logger, providers.Tracer(cfg.Service.Name))
	closeErr := closeLogWriter(ctx, w)
	restore()
	shutdownErr := providers.Shutdown(ctx)
	if err := errors.Join(emitErr, closeErr, shutdownErr); err != nil {
		return err
	}
	fmt.Println("exported OTLP:", endpoint)
	printTraceReport(report)
	return nil
}

func fileOptsFor(cfg blackboxConfig) []file.Option {
	if !cfg.Logs.Rotation.Enabled {
		return nil
	}
	return []file.Option{file.WithRotation(cfg.Logs.Rotation.fileConfig())}
}
func closeLogWriter(ctx context.Context, writer log.ManagedWriter) error {
	return writer.Close(ctx)
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
