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

// NewWriter creates the explicitly selected log output.
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

func (noopWriter) Write(context.Context, string, ...slog.Attr) error { return nil }
