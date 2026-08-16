package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestLayeredSpanTree 验证示例自身承诺的可观察行为（ADR-0023）：一次 POST
// /api/v1/orders 产生 server→handler→service→store→db 五层父子 span，共享同一
// trace_id；db 层带 db.system=mysql 属性。
func TestLayeredSpanTree(t *testing.T) {
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

	engine := newEngine()
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	spans := exporter.GetSpans()
	if len(spans) != 5 {
		t.Fatalf("span count = %d, want 5: %+v", len(spans), spans)
	}
	byName := make(map[string]tracetest.SpanStub, len(spans))
	var server *tracetest.SpanStub
	for i := range spans {
		s := spans[i]
		byName[s.Name] = s
		if s.SpanKind == trace.SpanKindServer {
			server = &s
		}
	}
	if server == nil {
		t.Fatal("缺 server span（SpanKindServer）")
	}
	if server.Name != "POST /api/v1/orders" {
		t.Fatalf("server span name = %q, want %q", server.Name, "POST /api/v1/orders")
	}

	chain := []string{"handler.order.Create", "service.order.Create", "store.order.Save", "db.order.insert"}
	prevSpan := *server
	for _, name := range chain {
		s, ok := byName[name]
		if !ok {
			t.Fatalf("缺 span %q", name)
		}
		if s.Parent.SpanID() != prevSpan.SpanContext.SpanID() {
			t.Fatalf("%s parent = %s, want %s", name, s.Parent.SpanID(), prevSpan.SpanContext.SpanID())
		}
		if s.SpanContext.TraceID() != server.SpanContext.TraceID() {
			t.Fatalf("%s trace = %s, want %s", name, s.SpanContext.TraceID(), server.SpanContext.TraceID())
		}
		prevSpan = s
	}

	if got := attrValue(byName["db.order.insert"].Attributes, "db.system"); got != "mysql" {
		t.Errorf("db span db.system = %q, want mysql", got)
	}
}

func attrValue(attrs []attribute.KeyValue, key string) string {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.Emit()
		}
	}
	return ""
}
