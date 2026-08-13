package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/writer/file"
	"github.com/formal-you/go-observability/writer/otlp"
	stdoutwriter "github.com/formal-you/go-observability/writer/stdout"
)

// NewWriter 按 Runtime 的 LogOutput 创建 ManagedWriter。
//
// file/stdout Writer 接收 Runtime 的 Resource identity；OTLP Writer 复用 Runtime 拥有的
// LoggerProvider，因此关闭 Writer 不会提前关闭 Provider。返回值的 Close 幂等，应用仍应
// 在 Runtime.Shutdown 前调用它，以释放 Writer 自己拥有的资源。
func (r *Runtime) NewWriter(ctx context.Context, cfg WriterConfig) (log.ManagedWriter, error) {
	if r == nil {
		return nil, errors.New("telemetry: nil runtime")
	}
	switch r.logOutput {
	case LogOutputFile:
		if strings.TrimSpace(cfg.FilePath) == "" {
			return nil, errors.New("telemetry: file output requires file path")
		}
		if len(cfg.StdoutOptions) != 0 {
			return nil, errors.New("telemetry: stdout options do not apply to file output")
		}
		// 复制调用方 slice 后追加进程级 metadata，避免修改调用方持有的配置。
		// metadata 选项最后应用，事件和调用方选项都不能覆盖 Runtime 的服务身份。
		opts := append([]file.Option(nil), cfg.FileOptions...)
		opts = append(opts, file.WithResourceMetadata(r.fileMetadata))
		writer, err := file.New(cfg.FilePath, opts...)
		if err != nil {
			return nil, err
		}
		return log.ManageWriter(writer), nil
	case LogOutputOTLP:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 || len(cfg.StdoutOptions) != 0 {
			return nil, errors.New("telemetry: writer options do not apply to otlp output")
		}
		if r.loggerProvider == nil {
			return nil, errors.New("telemetry: otlp output requires logger provider")
		}
		// 注入 Provider 表示 Runtime 保留所有权；Writer.Close 是 no-op，最终 Flush/Shutdown
		// 由 Runtime.Shutdown 统一完成。Logger.Emit 可被多个 goroutine 并发调用。
		writer, err := otlp.New(ctx, otlp.WithLoggerProvider(r.loggerProvider), otlp.WithResource(r.resource))
		if err != nil {
			return nil, err
		}
		return log.ManageWriter(writer), nil
	case LogOutputStdout:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 {
			return nil, errors.New("telemetry: file options do not apply to stdout output")
		}
		opts := append([]stdoutwriter.Option(nil), cfg.StdoutOptions...)
		opts = append(opts, stdoutwriter.WithResource(r.resource))
		writer, err := stdoutwriter.New(ctx, opts...)
		if err != nil {
			return nil, err
		}
		return log.ManageWriter(writer), nil
	case LogOutputNone:
		if cfg.FilePath != "" || len(cfg.FileOptions) != 0 || len(cfg.StdoutOptions) != 0 {
			return nil, errors.New("telemetry: writer options do not apply to none output")
		}
		return log.ManageWriter(noopWriter{}), nil
	default:
		return nil, fmt.Errorf("telemetry: invalid log output %q", r.logOutput)
	}
}

type noopWriter struct{}

// Write 实现显式禁用日志出口的无操作 Writer。
func (noopWriter) Write(context.Context, string, ...slog.Attr) error { return nil }
