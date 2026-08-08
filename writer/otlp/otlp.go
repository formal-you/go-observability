// Package otlp 提供基于 OTLP Logs 的 Writer：把事件写到 OTel 日志后端（Collector/Loki）。
// 采集频率由 opentelemetry-collector 的 batch processor 配置控制，本包只负责导出。
package otlp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/formal-you/go-observability/internal/attrkv"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	otelog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Writer 实现 log.Writer：把扁平 attrs 写入 OTLP Logs。
// trace_id/span_id 由 ctx 中的 span context 自动关联到 LogRecord。
type Writer struct {
	logger   otelog.Logger
	provider *sdklog.LoggerProvider
}

// Option 配置 New。
type Option func(*options)

type options struct {
	endpoint string
	res      *resource.Resource
}

// WithEndpoint 设置 OTLP gRPC endpoint；默认 127.0.0.1:4317。
func WithEndpoint(endpoint string) Option {
	return func(o *options) { o.endpoint = endpoint }
}

// WithResource 设置 Resource；默认 service.name=go-observability。
func WithResource(res *resource.Resource) Option {
	return func(o *options) { o.res = res }
}

// New 创建 OTLP Writer。
func New(ctx context.Context, opts ...Option) (*Writer, error) {
	o := options{endpoint: "127.0.0.1:4317"}
	for _, opt := range opts {
		opt(&o)
	}
	if o.res == nil {
		o.res = resource.NewWithAttributes(
			"https://opentelemetry.io/schemas/1.26.0",
			attribute.String("service.name", "go-observability"),
		)
	}
	exporter, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpointURL(o.endpoint))
	if err != nil {
		return nil, fmt.Errorf("otlp: create log exporter: %w", err)
	}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(o.res),
	)
	return &Writer{logger: provider.Logger("go-observability"), provider: provider}, nil
}

// Close 关闭 LoggerProvider，flush 待导出记录。
func (w *Writer) Close(ctx context.Context) error { return w.provider.Shutdown(ctx) }

// Write 把事件写为一条 OTLP LogRecord。
func (w *Writer) Write(ctx context.Context, msg string, attrs ...slog.Attr) error {
	rec := otelog.Record{}
	rec.SetTimestamp(time.Now())
	rec.SetObservedTimestamp(time.Now())
	severity, text := attrkv.Severity(attrs)
	rec.SetSeverity(severity)
	rec.SetSeverityText(text)
	rec.SetBody(otelog.StringValue(msg))
	rec.AddAttributes(attrkv.ToKeyValues(attrs)...)
	w.logger.Emit(ctx, rec)
	return nil
}
