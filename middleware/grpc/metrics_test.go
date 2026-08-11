package grpcmw

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMetricsRecordsDuration(t *testing.T) {
	_, reader := newTestMeter(t)
	interceptor := Metrics(MetricsConfig{})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Register"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}

	points := collectHistogram(t, reader, metricRPCServerDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if metricAttrString(t, points[0].Attributes, "rpc.service") != "mall.auth.v1.AuthService" {
		t.Fatalf("rpc.service = %q", metricAttrString(t, points[0].Attributes, "rpc.service"))
	}
	if metricAttrString(t, points[0].Attributes, "rpc.method") != "Register" {
		t.Fatalf("rpc.method = %q", metricAttrString(t, points[0].Attributes, "rpc.method"))
	}
	if metricAttrInt(t, points[0].Attributes, "rpc.grpc.status_code") != int64(codes.OK) {
		t.Fatalf("status attr = %d, want OK", metricAttrInt(t, points[0].Attributes, "rpc.grpc.status_code"))
	}
}

func TestMetricsErrorStatus(t *testing.T) {
	_, reader := newTestMeter(t)
	interceptor := Metrics(MetricsConfig{})
	info := &grpc.UnaryServerInfo{FullMethod: "/mall.auth.v1.AuthService/Login"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.InvalidArgument, "bad")
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
	points := collectHistogram(t, reader, metricRPCServerDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if metricAttrInt(t, points[0].Attributes, "rpc.grpc.status_code") != int64(codes.InvalidArgument) {
		t.Fatalf("status attr = %d, want InvalidArgument", metricAttrInt(t, points[0].Attributes, "rpc.grpc.status_code"))
	}
}
