package telemetry

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
)

// InstallGlobal installs this Runtime's non-nil providers and the W3C
// propagator. The returned restore function is idempotent. Nested installs are
// supported when their restore functions are called in LIFO order.
func (r *Runtime) InstallGlobal() func() {
	if r == nil || (r.tracerProvider == nil && r.meterProvider == nil && r.loggerProvider == nil) {
		return func() {}
	}
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
	r.restoreMu.Lock()
	r.restores = append(r.restores, restore)
	r.restoreMu.Unlock()
	return restore
}

func w3cPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

// Shutdown restores globals first, then shuts down log, metric, and trace.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.shutdown.Do(func() {
		r.restoreMu.Lock()
		for i := len(r.restores) - 1; i >= 0; i-- {
			r.restores[i]()
		}
		r.restoreMu.Unlock()
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
