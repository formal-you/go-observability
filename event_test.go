package log

import (
	"log/slog"
	"net"
	"testing"
	"time"
)

func attrMap(attrs []slog.Attr) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestAccessEventAttrsConform(t *testing.T) {
	ev := AccessEvent{
		EventMetadata: EventMetadata{
			Timestamp: time.Now(),
			Level:     LevelInfo,
			TraceID:   "0123456789abcdef0123456789abcdef",
			SpanID:    "0123456789abcdef",
			RequestID: "req-001",
			LatencyMS: 12,
		},
		Data: AccessPayload{
			EventName: "access.http.request",
			Subject:   Subject{UserID: "u1"},
			HTTP: HTTPInfo{
				Method:     "POST",
				URLPath:    "/api/v1/orders",
				Route:      "POST /api/v1/orders",
				StatusCode: 201,
				ClientIP:   net.ParseIP("127.0.0.1"),
				UserAgent:  "curl/8",
			},
			Result: ResultSuccess,
		},
	}
	attrs := attrMap(ev.Attrs())
	want := []string{
		"timestamp", "level", "request_id", "latency_ms", "trace_id", "span_id",
		"event.name",
		"http.request.method", "http.request.path", "http.route", "http.response.status_code",
		"client.address", "user_agent.original",
		"mall.user_id", "mall.result",
	}
	for _, k := range want {
		if _, ok := attrs[k]; !ok {
			t.Errorf("缺少属性 %s（实际: %v）", k, keysOf(attrs))
		}
	}
	for _, k := range []string{"service.name", "deployment.environment"} {
		if _, ok := attrs[k]; ok {
			t.Errorf("不应出现保留键 %s", k)
		}
	}
}

func TestErrorEventKeepsFalseRetryable(t *testing.T) {
	ev := ErrorEvent{
		EventMetadata: EventMetadata{Level: LevelError},
		Data: ErrorPayload{
			EventName:    "error.system.db.timeout",
			ErrorType:    "db.timeout",
			ErrorMessage: "dial tcp: connection refused",
			Retryable:    false,
			Source:       Source{Function: "orderRepo.FindByID", Filepath: "order/repo.go", Line: 42},
			Result:       ResultError,
		},
	}
	attrs := attrMap(ev.Attrs())
	if _, ok := attrs["mall.retryable"]; !ok {
		t.Error("布尔 false 不应被省略（mall.retryable 必须保留）")
	}
	if got, ok := attrs["error.type"].(slog.Value); !ok || got.String() != "db.timeout" {
		t.Errorf("error.type = %v, want db.timeout", attrs["error.type"])
	}
	if _, ok := attrs["code.lineno"]; !ok {
		t.Error("code.lineno 应输出（Line=42 非零）")
	}
}

func TestZeroValueOmission(t *testing.T) {
	ev := BusinessEvent{
		EventMetadata: EventMetadata{Level: LevelInfo},
		Data: BusinessPayload{
			EventName: "business.order.paid",
			Result:    ResultSuccess,
		},
	}
	attrs := attrMap(ev.Attrs())
	for _, k := range []string{"mall.user_id", "mall.business_code", "latency_ms"} {
		if _, ok := attrs[k]; ok {
			t.Errorf("零值字段不应输出：%s", k)
		}
	}
}

// TestMiddlewareTypeSwitch 演示中间件按具体事件类型分发（类型断言）。
func TestMiddlewareTypeSwitch(t *testing.T) {
	var ev EventPayload = AccessEvent{
		EventMetadata: EventMetadata{Level: LevelInfo},
		Data: AccessPayload{
			EventName: "access.http.request",
			HTTP:      HTTPInfo{Method: "GET", StatusCode: 200},
			Result:    ResultSuccess,
		},
	}
	switch e := ev.(type) {
	case AccessEvent:
		if e.Data.HTTP.StatusCode != 200 {
			t.Errorf("status = %d, want 200", e.Data.HTTP.StatusCode)
		}
		if e.Level != LevelInfo {
			t.Errorf("level = %s, want INFO（embed 字段提升）", e.Level)
		}
	case ErrorEvent:
		t.Error("不应断言为 ErrorEvent")
	default:
		t.Fatalf("未知事件类型: %T", ev)
	}
}
