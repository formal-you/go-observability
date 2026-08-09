// Package otlp 提供基于 OTLP Logs 的 Writer：把事件写到 OTel 日志后端（Collector/Loki）。
// 采集频率由 opentelemetry-collector 的 batch processor 配置控制，本包只负责导出。
package otlp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/formal-you/go-observability/internal/attrkv"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	otelog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Writer 实现 log.Writer：把扁平 attrs 写入 OTLP Logs。
// trace_id/span_id 由 ctx 中的 span context 自动关联到 LogRecord（sdk/log 的 Emit 行为），
// 事件 metadata 中的同名键不会写成属性。
type Writer struct {
	logger       otelog.Logger
	provider     *sdklog.LoggerProvider
	ownsProvider bool
}

// Option 配置 New。
type Option func(*options)

type options struct {
	endpoint string
	res      *resource.Resource
	provider *sdklog.LoggerProvider
}

// WithLoggerProvider 注入外部 LoggerProvider（如 internal/telemetry 装配的三信号 provider）。
// 注入时 Writer 不拥有该 provider，Close 不会 Shutdown 它（由装配层统一关闭）。
func WithLoggerProvider(provider *sdklog.LoggerProvider) Option {
	return func(o *options) { o.provider = provider }
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
			"https://opentelemetry.io/schemas/1.41.0",
			attribute.String("service.name", "go-observability"),
		)
	}
	if o.provider == nil {
		exporter, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpointURL(o.endpoint))
		if err != nil {
			return nil, fmt.Errorf("otlp: create log exporter: %w", err)
		}
		o.provider = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
			sdklog.WithResource(o.res),
		)
	}
	return &Writer{logger: o.provider.Logger("go-observability"), provider: o.provider, ownsProvider: true}, nil
}

// Close 关闭自建的 LoggerProvider，flush 待导出记录。
// 注入外部 provider（WithLoggerProvider）时 Close 为 no-op，由装配层统一 Shutdown。
func (w *Writer) Close(ctx context.Context) error {
	if !w.ownsProvider {
		return nil
	}
	return w.provider.Shutdown(ctx)
}

// Write 把事件写为一条 OTLP LogRecord。
// timestamp/level 映射为 LogRecord 顶层字段；trace_id/span_id 由 ctx 的 span context 关联，
// 不写入属性。采集频率由 opentelemetry-collector 的 batch processor 控制，本方法同步 Emit 即返回。
func (w *Writer) Write(ctx context.Context, msg string, attrs ...slog.Attr) error {
	rec, rest := attrkv.Record(msg, attrs)
	rec.AddAttributes(attrkv.ToKeyValues(rest)...)
	w.logger.Emit(ctx, rec)
	return nil
}
