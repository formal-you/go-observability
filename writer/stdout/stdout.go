// Package stdout 提供基于 stdoutlog exporter 的 Writer（本地演示）。
package stdout

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	otelog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/formal-you/go-observability/internal/attrkv"
)

// Writer 实现 log.Writer：把事件输出到指定 io.Writer（默认 stdout）。
type Writer struct {
	logger   otelog.Logger
	provider *sdklog.LoggerProvider
}

// Option 配置 New。
type Option func(*options)

type options struct {
	output io.Writer
}

// WithOutput 设置输出目标；默认 os.Stdout。
func WithOutput(w io.Writer) Option {
	return func(o *options) { o.output = w }
}

// New 创建 stdout Writer。
func New(ctx context.Context, opts ...Option) (*Writer, error) {
	o := options{output: os.Stdout}
	for _, opt := range opts {
		opt(&o)
	}
	exporter, err := stdoutlog.New(stdoutlog.WithWriter(o.output))
	if err != nil {
		return nil, err
	}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	return &Writer{logger: provider.Logger("go-observability"), provider: provider}, nil
}

// Close 关闭 LoggerProvider，flush 待导出记录。
func (w *Writer) Close(ctx context.Context) error { return w.provider.Shutdown(ctx) }

// Write 把事件写为一条 stdout LogRecord；与 otlp.Writer 共享同一组装逻辑：
// timestamp/level 映射顶层字段，trace_id/span_id 由 ctx 关联，不写成属性。
func (w *Writer) Write(ctx context.Context, msg string, attrs ...slog.Attr) error {
	rec, rest := attrkv.Record(msg, attrs)
	rec.AddAttributes(attrkv.ToKeyValues(rest)...)
	w.logger.Emit(ctx, rec)
	return nil
}
