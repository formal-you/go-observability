package log

import (
	"context"
	"log/slog"
	"testing"
)

type benchmarkWriter struct{}

func (benchmarkWriter) Write(context.Context, string, ...slog.Attr) error { return nil }

func benchmarkEvent() BusinessEvent {
	return BusinessEvent{
		EventMetadata: EventMetadata{Level: LevelInfo, TraceID: "0123456789abcdef0123456789abcdef"},
		Data: BusinessPayload{
			EventName:       "order.payment.succeeded",
			Subject:         Subject{UserID: "user-1", TenantID: "tenant-1"},
			ErrorType:       "business.payment_failed",
			ErrorCode:       "ORDER.PAYMENT.SUCCEEDED",
			BusinessMessage: "payment accepted",
			Result:          ResultSuccess,
			ExtraAttrs: []slog.Attr{
				slog.String("app.order_id", "ORD-1001"),
				slog.Int64("app.amount_cents", 9900),
			},
		},
	}
}

func BenchmarkLoggerEmit(b *testing.B) {
	logger := NewLogger(benchmarkWriter{})
	event := benchmarkEvent()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Emit(context.Background(), event)
	}
}

func BenchmarkLoggerEmitMaskSample(b *testing.B) {
	logger := NewLogger(benchmarkWriter{},
		WithMasker(FieldMasker{}),
		WithSampler(ResultKeepSampler{Ratio: 1}),
	)
	event := benchmarkEvent()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Emit(context.Background(), event)
	}
}
