package log

import (
	"context"
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
			EventName: EventNameAccessHTTPRequest,
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
		"http.request.method", "url.path", "http.route", "http.response.status_code",
		"client.address", "user_agent.original",
		"app.user_id", "app.result",
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
			EventName:    EventNameErrorDBTimeout,
			ErrorType:    "db.timeout",
			ErrorMessage: "dial tcp: connection refused",
			Retryable:    false,
			Source:       Source{Function: "orderRepo.FindByID", Filepath: "order/repo.go", Line: 42},
			Result:       ResultError,
		},
	}
	attrs := attrMap(ev.Attrs())
	if _, ok := attrs["app.retryable"]; !ok {
		t.Error("布尔 false 不应被省略（app.retryable 必须保留）")
	}
	if got, ok := attrs["error.type"].(slog.Value); !ok || got.String() != "db.timeout" {
		t.Errorf("error.type = %v, want db.timeout", attrs["error.type"])
	}
	if _, ok := attrs["code.line.number"]; !ok {
		t.Error("code.line.number 应输出（Line=42 非零）")
	}
}

func TestZeroValueOmission(t *testing.T) {
	ev := BusinessEvent{
		EventMetadata: EventMetadata{Level: LevelInfo},
		Data: BusinessPayload{
			EventName: EventName("business.order.paid"),
			Result:    ResultSuccess,
		},
	}
	attrs := attrMap(ev.Attrs())
	for _, k := range []string{"app.user_id", "app.business_code", "latency_ms"} {
		if _, ok := attrs[k]; ok {
			t.Errorf("零值字段不应输出：%s", k)
		}
	}
}

func TestBusinessPayloadExtraAttrsCannotOverrideGovernanceKeys(t *testing.T) {
	payload := BusinessPayload{
		EventName:       EventName("business.order.paid"),
		ErrorType:       "business.payment_failed",
		Subject:         Subject{UserID: "user-canonical", TenantID: "tenant-canonical"},
		Resource:        Resource{Type: "order", ID: "order-canonical"},
		BusinessCode:    "ORDER.PAYMENT.FAILED",
		BusinessMessage: "payment failed",
		Source:          Source{Function: "checkout.Pay", Filepath: "checkout/pay.go", Line: 42},
		Result:          ResultFailed,
		ExtraAttrs: []slog.Attr{
			slog.String(string(KeyEventName), "business.order.forged"),
			slog.String(string(KeyErrorType), "forged.error"),
			slog.String(string(KeyAppUserID), "user-forged"),
			slog.String(string(KeyAppTenantID), "tenant-forged"),
			slog.String(string(KeyAppResourceType), "forged-resource"),
			slog.String(string(KeyAppResourceID), "resource-forged"),
			slog.String(string(KeyAppBusinessCode), "FORGED"),
			slog.String(string(KeyAppBusinessMessage), "forged"),
			slog.String(string(KeyCodeFunctionName), "forged.Function"),
			slog.String(string(KeyCodeFilePath), "forged.go"),
			slog.Int(string(KeyCodeLineNumber), 999),
			slog.String(string(KeyAppResult), string(ResultSuccess)),
			slog.String(string(KeyTimestamp), "forged"),
			slog.String("service.name", "forged"),
			slog.String("app.order_id", "ORD-1"),
		},
	}

	attrs := payload.Attrs()
	counts := make(map[string]int)
	values := make(map[string]string)
	for _, attr := range attrs {
		counts[attr.Key]++
		values[attr.Key] = attr.Value.String()
	}
	wantCanonical := map[string]string{
		string(KeyEventName):          string(payload.EventName),
		string(KeyErrorType):          payload.ErrorType,
		string(KeyAppUserID):          payload.Subject.UserID,
		string(KeyAppTenantID):        payload.Subject.TenantID,
		string(KeyAppResourceType):    payload.Resource.Type,
		string(KeyAppResourceID):      payload.Resource.ID,
		string(KeyAppBusinessCode):    payload.BusinessCode,
		string(KeyAppBusinessMessage): payload.BusinessMessage,
		string(KeyCodeFunctionName):   payload.Source.Function,
		string(KeyCodeFilePath):       payload.Source.Filepath,
		string(KeyCodeLineNumber):     "42",
		string(KeyAppResult):          string(payload.Result),
	}
	for key, want := range wantCanonical {
		if counts[key] != 1 || values[key] != want {
			t.Errorf("%s 未保持唯一 canonical 值: count=%d value=%q want=%q", key, counts[key], values[key], want)
		}
	}
	for _, key := range []string{string(KeyTimestamp), "service.name"} {
		if counts[key] != 0 {
			t.Errorf("ExtraAttrs 保留键 %s 未过滤", key)
		}
	}
	if values["app.order_id"] != "ORD-1" {
		t.Errorf("合法 ExtraAttrs 丢失: %v", values)
	}

	sampler := ResultKeepSampler{Ratio: 0}
	if !sampler.Sample(context.Background(), attrs) {
		t.Error("伪造 app.result 不应绕过 failed 强制保留规则")
	}
}

// TestMiddlewareTypeSwitch 演示中间件按具体事件类型分发（类型断言）。
func TestMiddlewareTypeSwitch(t *testing.T) {
	var ev EventPayload = AccessEvent{
		EventMetadata: EventMetadata{Level: LevelInfo},
		Data: AccessPayload{
			EventName: EventNameAccessHTTPRequest,
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

// TestEventNameConventions 验证 EventName 注册表符合 类别.模块.操作 三段式，且构造校验生效。
func TestEventNameConventions(t *testing.T) {
	names := []EventName{
		EventNameAccessHTTPRequest,
		EventName("business.order.paid"),
		EventNameErrorDBTimeout,
	}
	for _, n := range names {
		if err := n.Validate(); err != nil {
			t.Errorf("%s 不符合段式: %v", n, err)
		}
	}
	if got := NewEventName("access", "http", "request"); got != EventNameAccessHTTPRequest {
		t.Errorf("NewEventName = %q, want %q", got, EventNameAccessHTTPRequest)
	}

	assertPanics := func(name string, f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: 期望 panic，实际未 panic", name)
			}
		}()
		f()
	}
	assertPanics("2 段", func() { NewEventName("access", "http") })
	assertPanics("4 段", func() { NewEventName("access", "http", "a", "b") })
	assertPanics("大写", func() { NewEventName("Access", "http", "request") })
	if err := EventName("error.system.db.timeout").Validate(); err == nil {
		t.Error("4 段事件名应校验失败")
	}
}

// TestRequestIDDerivedFromTraceID 验证 A4：RequestID 为空且 TraceID 非空时，归一化派生
// trace_id 前缀（12 hex）；显式 RequestID 优先；trace 不足长度时原样兜底。
func TestRequestIDDerivedFromTraceID(t *testing.T) {
	traceID := "0123456789abcdef0123456789abcdef"

	derive := func(md EventMetadata) string {
		ev := AccessEvent{
			EventMetadata: md,
			Data: AccessPayload{
				EventName: EventNameAccessHTTPRequest,
				HTTP:      HTTPInfo{Method: "GET", StatusCode: 200},
				Result:    ResultSuccess,
			},
		}
		attrs := attrMap(ev.Attrs())
		v, ok := attrs["request_id"].(slog.Value)
		if !ok {
			return ""
		}
		return v.String()
	}

	if got := derive(EventMetadata{TraceID: traceID}); got != "0123456789ab" {
		t.Errorf("派生 request_id = %q, want 0123456789ab（trace_id 前 12 hex）", got)
	}
	if got := derive(EventMetadata{TraceID: traceID, RequestID: "REQ-GW-1"}); got != "REQ-GW-1" {
		t.Errorf("显式 request_id 应优先, got %q", got)
	}
	if got := derive(EventMetadata{TraceID: "abcd"}); got != "abcd" {
		t.Errorf("短 trace_id 应原样兜底, got %q", got)
	}
	if got := derive(EventMetadata{}); got != "" {
		t.Errorf("无 trace 无显式值应省略, got %q", got)
	}
}
