package otelutil

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/middleware/internal/mwutil"
)

// SpanOption 配置 StartSpan / WithSpan 创建的 span；只能通过本包构造函数获得。
type SpanOption interface {
	applySpanOption(*spanOptionConfig)
}

// spanOptionConfig 聚合 StartSpan / WithSpan 的 span 创建参数。
type spanOptionConfig struct {
	tracer trace.Tracer
	start  []trace.SpanStartOption
}

// spanOptionFunc 允许以函数形式实现 SpanOption。
type spanOptionFunc func(*spanOptionConfig)

func (f spanOptionFunc) applySpanOption(c *spanOptionConfig) { f(c) }

// WithTracer 指定创建 span 使用的 Tracer；nil 时使用全局 otel.Tracer("go-observability")。
// 对齐 TraceConfig 的 nil→全局 约定：显式注入便于在测试与多 Provider 场景隔离信号。
func WithTracer(t trace.Tracer) SpanOption {
	return spanOptionFunc(func(c *spanOptionConfig) { c.tracer = t })
}

// WithStartOption 转发一个 trace.SpanStartOption（如 trace.WithAttributes、
// trace.WithSpanKind、trace.WithLinks）到 StartSpan / WithSpan 的 tracer.Start。
func WithStartOption(opt trace.SpanStartOption) SpanOption {
	return spanOptionFunc(func(c *spanOptionConfig) { c.start = append(c.start, opt) })
}

// StartSpan 创建名为 name 的 span，返回带新 span 的 ctx 供继续下传。
// tracer 默认全局 otel.Tracer("go-observability")，可用 WithTracer 注入；
// span 生命周期（End / SetStatus / RecordError）由调用方负责，适合需要中途
// 写属性或分段结束的场景。span name 是生命周期建模（非 EventName），由调用方
// 按操作语义命名。StartSpan 可并发调用，span 的并发安全由 OTel SDK 保证。
func StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, trace.Span) {
	cfg := spanOptionConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt.applySpanOption(&cfg)
		}
	}
	tracer := cfg.tracer
	if tracer == nil {
		tracer = otel.Tracer("go-observability")
	}
	return tracer.Start(ctx, name, cfg.start...)
}

// WithSpan 把 fn 包成一个 span（同步包装）：自动 Start/End；fn 返回 err 时
// SetStatus(codes.Error) 并 RecordError(err)；fn panic 时标记 Error 后重抛
// （与 mwutil.FinishSpan 语义一致）。返回带新 span 的 ctx 供继续下传，调用方
// 必须把返回的 ctx 继续传给下层，子 span 才能挂到本 span 之下。
// fn 为 nil 时返回错误且不创建 span；span name 是生命周期建模（非 EventName）。
func WithSpan(ctx context.Context, name string, fn func(ctx context.Context) error, opts ...SpanOption) (context.Context, error) {
	if fn == nil {
		return ctx, errors.New("otelutil: WithSpan: fn is nil")
	}
	ctx, span := StartSpan(ctx, name, opts...)
	defer mwutil.FinishSpan(span)
	if err := fn(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return ctx, err
	}
	return ctx, nil
}
