package otelutil_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/middleware/otelutil"
)

// newTestProvider 返回 AlwaysSample 的内存 exporter + TracerProvider，作为黑盒观察点。
func newTestProvider(t *testing.T) (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return exporter, tp
}

func findSpan(stubs []tracetest.SpanStub, name string) *tracetest.SpanStub {
	for i := range stubs {
		if stubs[i].Name == name {
			return &stubs[i]
		}
	}
	return nil
}

// TestWithSpanCreatesChildSpan 验证 WithSpan 把 fn 包成根 span 的子 span：父子关系、
// 同 trace、返回 ctx 携带新 span，且 span 已结束（能被 exporter 观察到）。
func TestWithSpanCreatesChildSpan(t *testing.T) {
	exporter, tp := newTestProvider(t)
	tracer := tp.Tracer("blackbox")
	rootCtx, root := tracer.Start(context.Background(), "root")

	ctx, err := otelutil.WithSpan(rootCtx, "service.user.Get", func(ctx context.Context) error {
		return nil
	}, otelutil.WithTracer(tracer))
	if err != nil {
		t.Fatalf("WithSpan err = %v", err)
	}
	child := trace.SpanFromContext(ctx)
	if !child.SpanContext().IsValid() {
		t.Fatal("返回 ctx 应携带有效子 span")
	}
	root.End()

	childStub := findSpan(exporter.GetSpans(), "service.user.Get")
	if childStub == nil {
		t.Fatal("缺 service.user.Get span")
	}
	if childStub.Parent.SpanID() != root.SpanContext().SpanID() {
		t.Fatalf("child parent = %s, want root %s", childStub.Parent.SpanID(), root.SpanContext().SpanID())
	}
	if childStub.SpanContext.TraceID() != root.SpanContext().TraceID() {
		t.Fatal("子 span 应共享 root 的 trace id")
	}
}

// TestWithSpanError 验证 fn 返回 err 时：原样返回、span 状态 Error 且带 exception 事件。
func TestWithSpanError(t *testing.T) {
	exporter, tp := newTestProvider(t)
	sentinel := errors.New("store unavailable")
	ctx, err := otelutil.WithSpan(context.Background(), "store.user.FindByID", func(ctx context.Context) error {
		return sentinel
	}, otelutil.WithTracer(tp.Tracer("blackbox")))
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
	if sc := trace.SpanFromContext(ctx).SpanContext(); !sc.IsValid() {
		t.Fatal("返回 ctx 应携带子 span")
	}
	stub := findSpan(exporter.GetSpans(), "store.user.FindByID")
	if stub == nil {
		t.Fatal("缺 store.user.FindByID span")
	}
	if stub.Status.Code != codes.Error {
		t.Fatalf("status = %v, want Error", stub.Status.Code)
	}
	if len(stub.Events) == 0 || stub.Events[0].Name != "exception" {
		t.Fatalf("events = %+v, want 含 exception（RecordError）", stub.Events)
	}
}

// TestWithSpanPanic 验证 fn panic 时：span 标记 Error、已结束，且 panic 被重抛。
func TestWithSpanPanic(t *testing.T) {
	exporter, tp := newTestProvider(t)
	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("WithSpan 应重抛 panic")
			}
		}()
		_, _ = otelutil.WithSpan(context.Background(), "service.user.Get", func(ctx context.Context) error {
			panic("boom")
		}, otelutil.WithTracer(tp.Tracer("blackbox")))
	}()
	stub := findSpan(exporter.GetSpans(), "service.user.Get")
	if stub == nil {
		t.Fatal("缺 service.user.Get span")
	}
	if stub.Status.Code != codes.Error {
		t.Fatalf("status = %v, want Error", stub.Status.Code)
	}
}

// TestStartSpanPropagatesContext 验证 StartSpan 返回的新 ctx 可继续创建子 span（层级下传）。
func TestStartSpanPropagatesContext(t *testing.T) {
	exporter, tp := newTestProvider(t)
	tracer := tp.Tracer("blackbox")
	ctx, outer := otelutil.StartSpan(context.Background(), "handler.order.Create", otelutil.WithTracer(tracer))
	_, inner := otelutil.StartSpan(ctx, "service.order.Create", otelutil.WithTracer(tracer))
	outer.End()
	inner.End()

	outerStub := findSpan(exporter.GetSpans(), "handler.order.Create")
	innerStub := findSpan(exporter.GetSpans(), "service.order.Create")
	if outerStub == nil || innerStub == nil {
		t.Fatal("缺 handler/service span")
	}
	if innerStub.Parent.SpanID() != outerStub.SpanContext.SpanID() {
		t.Fatalf("inner parent = %s, want outer %s", innerStub.Parent.SpanID(), outerStub.SpanContext.SpanID())
	}
}

// TestWithTracerInjection 验证 WithTracer 注入生效：span 落在注入 Tracer 的
// instrumentation scope（custom-tracer）下。
func TestWithTracerInjection(t *testing.T) {
	exporter, tp := newTestProvider(t)
	_, err := otelutil.WithSpan(context.Background(), "db.query", func(ctx context.Context) error {
		return nil
	}, otelutil.WithTracer(tp.Tracer("custom-tracer")))
	if err != nil {
		t.Fatalf("WithSpan err = %v", err)
	}
	stub := findSpan(exporter.GetSpans(), "db.query")
	if stub == nil {
		t.Fatal("缺 db.query span")
	}
	if got := stub.InstrumentationScope.Name; got != "custom-tracer" {
		t.Fatalf("instrumentation scope = %q, want custom-tracer", got)
	}
}

// TestWithSpanDefaultUsesGlobalTracer 验证不注入时使用全局 otel.Tracer("go-observability")。
func TestWithSpanDefaultUsesGlobalTracer(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	_, err := otelutil.WithSpan(context.Background(), "default.global", func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithSpan err = %v", err)
	}
	if stub := findSpan(exporter.GetSpans(), "default.global"); stub == nil {
		t.Fatal("缺 default.global span（应走全局 TracerProvider）")
	}
}

// TestWithSpanNilFn 验证 nil fn 返回错误且不创建 span（请求路径不 panic）。
func TestWithSpanNilFn(t *testing.T) {
	exporter, tp := newTestProvider(t)
	ctx, err := otelutil.WithSpan(context.Background(), "noop", nil, otelutil.WithTracer(tp.Tracer("blackbox")))
	if err == nil {
		t.Fatal("nil fn 应返回错误")
	}
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		t.Fatal("nil fn 不应创建 span")
	}
	if len(exporter.GetSpans()) != 0 {
		t.Fatal("nil fn 不应产生 span")
	}
}
