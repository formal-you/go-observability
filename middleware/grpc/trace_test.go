package grpcmw

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestTraceCreatesSpan(t *testing.T) {
	tracer, p := newTestTracer(t)
	interceptor := Trace(TraceConfig{Tracer: tracer})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Register"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}

	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	span := p.spans[0]
	if span.Name() != info.FullMethod {
		t.Fatalf("span name = %q, want FullMethod", span.Name())
	}
	if span.SpanKind() != trace.SpanKindServer {
		t.Fatalf("span kind = %v, want Server", span.SpanKind())
	}
	if attrStr(t, span, "rpc.service") != "mall.auth.v1.AuthService" {
		t.Fatalf("rpc.service = %s", attrStr(t, span, "rpc.service"))
	}
	if attrStr(t, span, "rpc.method") != "Register" {
		t.Fatalf("rpc.method = %s", attrStr(t, span, "rpc.method"))
	}
	if attrInt(t, span, "rpc.grpc.status_code") != int64(grpccodes.OK) {
		t.Fatalf("status attr = %d", attrInt(t, span, "rpc.grpc.status_code"))
	}
}

func TestTraceMarksError(t *testing.T) {
	tracer, p := newTestTracer(t)
	interceptor := Trace(TraceConfig{Tracer: tracer})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Login"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(grpccodes.InvalidArgument, "bad")
	})
	if status.Code(err) != grpccodes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	if p.spans[0].Status().Code != codes.Error {
		t.Fatalf("span status = %v, want Error（非 OK）", p.spans[0].Status())
	}
	if attrInt(t, p.spans[0], "rpc.grpc.status_code") != int64(grpccodes.InvalidArgument) {
		t.Fatalf("status attr = %d", attrInt(t, p.spans[0], "rpc.grpc.status_code"))
	}
}

const (
	testTraceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	testTraceID     = "4bf92f3577b34da6a3ce929d0e0e4736"
)

// TestTraceExtractsPropagation 验证 gRPC 入口提取：metadata 带 traceparent 时接续调用方链路。
func TestTraceExtractsPropagation(t *testing.T) {
	tracer, p := newTestTracer(t)
	interceptor := Trace(TraceConfig{Tracer: tracer})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Register"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("traceparent", testTraceParent))
	_, err := interceptor(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	if got := p.spans[0].Parent().TraceID().String(); got != testTraceID {
		t.Fatalf("span parent traceID = %s, want %s", got, testTraceID)
	}
}
