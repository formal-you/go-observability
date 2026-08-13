package telemetry

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
)

// InstallGlobal 把 Runtime 中非 nil 的 Provider 和 W3C Trace Context/Baggage
// Propagator 安装为进程级 OpenTelemetry global state。
//
// 返回的 restore 函数使用 sync.Once 保证幂等，并恢复安装前捕获的对象。嵌套安装只在
// 调用方按 LIFO 顺序恢复时有定义。该方法用于应用启动/退出阶段；调用方不得让
// InstallGlobal 与 Shutdown 并发执行，OTel global setter 本身不能替代应用级生命周期协调。
func (r *Runtime) InstallGlobal() func() {
	if r == nil || (r.tracerProvider == nil && r.meterProvider == nil && r.loggerProvider == nil) {
		return func() {}
	}
	// 在写入任何 global state 前保存完整快照，使 restore 能恢复到调用前状态。
	oldTrace := otel.GetTracerProvider()
	oldMeter := otel.GetMeterProvider()
	oldLogger := global.GetLoggerProvider()
	oldPropagator := otel.GetTextMapPropagator()
	if r.tracerProvider != nil {
		otel.SetTracerProvider(r.tracerProvider)
	}
	if r.meterProvider != nil {
		otel.SetMeterProvider(r.meterProvider)
	}
	if r.loggerProvider != nil {
		global.SetLoggerProvider(r.loggerProvider)
	}
	otel.SetTextMapPropagator(w3cPropagator())

	// 每个安装动作拥有独立 Once；重复调用同一个 restore 不会重复覆盖后续状态。
	var once sync.Once
	restore := func() {
		once.Do(func() {
			if r.loggerProvider != nil {
				global.SetLoggerProvider(oldLogger)
			}
			if r.meterProvider != nil {
				otel.SetMeterProvider(oldMeter)
			}
			if r.tracerProvider != nil {
				otel.SetTracerProvider(oldTrace)
			}
			otel.SetTextMapPropagator(oldPropagator)
		})
	}
	// Runtime 记录所有未必由调用方显式执行的 restore，Shutdown 会统一兜底恢复。
	r.restoreMu.Lock()
	r.restores = append(r.restores, restore)
	r.restoreMu.Unlock()
	return restore
}

func w3cPropagator() propagation.TextMapPropagator {
	// TraceContext 传播 TraceID/SpanID，Baggage 传播经应用明确允许的跨服务上下文。
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

// Shutdown 恢复 Runtime 安装的 global state，并依次关闭 Log、Metric、Trace Provider。
//
// 关闭顺序保证待导出的日志和指标仍可引用有效的 Trace Context。方法使用 sync.Once，
// 可重复或并发调用；首次调用的 context 和结果决定后续返回值。调用方应提供带合理
// deadline 的 context，以便 BatchProcessor Flush 后退出。
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.shutdown.Do(func() {
		// 逆序执行与嵌套 InstallGlobal 的 LIFO 契约一致。
		r.restoreMu.Lock()
		for i := len(r.restores) - 1; i >= 0; i-- {
			r.restores[i]()
		}
		r.restoreMu.Unlock()
		// 每个 Provider 都获得关闭机会，errors.Join 保留全部失败原因。
		var errs []error
		if r.loggerProvider != nil {
			logErr := r.loggerProvider.Shutdown(ctx)
			if logErr != nil && r.counters != nil {
				// LoggerProvider can return a context error before its processor
				// reaches the exporter, so the exporter wrapper cannot observe it.
				r.counters.logExportErrors.Add(1)
			}
			errs = appendError(errs, logErr)
		}
		if r.meterProvider != nil {
			errs = appendError(errs, r.meterProvider.Shutdown(ctx))
		}
		if r.tracerProvider != nil {
			errs = appendError(errs, r.tracerProvider.Shutdown(ctx))
		}
		r.shutdownErr = errors.Join(errs...)
	})
	return r.shutdownErr
}

func appendError(errs []error, err error) []error {
	if err != nil {
		return append(errs, err)
	}
	return errs
}
