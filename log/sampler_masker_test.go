package log

import (
	"context"
	"log/slog"
	"testing"
)

type nestedLogValue struct{}

func (nestedLogValue) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("profile", map[string]any{
			"credentials": Fields{"password": "from-log-valuer"},
		}),
	)
}

func TestResultKeepSamplerHighValue(t *testing.T) {
	s := ResultKeepSampler{Ratio: 0}
	for _, r := range []Result{ResultFailed, ResultError, ResultBlocked, ResultDenied} {
		attrs := []slog.Attr{slog.String(string(KeyAppResult), string(r))}
		if !s.Sample(context.Background(), attrs) {
			t.Errorf("result %s 应强制保留", r)
		}
	}
	if s.Sample(context.Background(), []slog.Attr{slog.String(string(KeyAppResult), string(ResultSuccess))}) {
		t.Error("Ratio=0 时 success 应丢弃")
	}
}

func TestResultKeepSamplerRatio(t *testing.T) {
	calls := 0
	s := ResultKeepSampler{
		Ratio: 0.5,
		randFloat: func() float64 {
			calls++
			if calls == 1 {
				return 0.1 // < 0.5 keep
			}
			return 0.9 // drop
		},
	}
	attrs := []slog.Attr{slog.String(string(KeyAppResult), string(ResultSuccess))}
	if !s.Sample(context.Background(), attrs) {
		t.Error("rand 0.1 应保留")
	}
	if s.Sample(context.Background(), attrs) {
		t.Error("rand 0.9 应丢弃")
	}
}

func TestFieldMasker(t *testing.T) {
	m := FieldMasker{Keys: []string{"app.custom_secret", string(KeyAppUserID)}}
	in := []slog.Attr{
		slog.String("password", "x"),
		slog.String(string(KeyAppUserID), "u1"),
		slog.String("app.custom_secret", "s"),
		slog.String("http.request.method", "GET"),
		slog.Group("nested", slog.String("token", "t")),
	}
	out := m.Mask(context.Background(), in)
	got := map[string]string{}
	var walk func([]slog.Attr, string)
	walk = func(as []slog.Attr, prefix string) {
		for _, a := range as {
			k := a.Key
			if prefix != "" {
				k = prefix + "." + a.Key
			}
			if a.Value.Kind() == slog.KindGroup {
				walk(a.Value.Group(), k)
				continue
			}
			got[k] = a.Value.String()
		}
	}
	walk(out, "")
	if got["password"] != "***" || got[string(KeyAppUserID)] != "***" || got["app.custom_secret"] != "***" {
		t.Errorf("敏感键未掩码: %v", got)
	}
	if got["http.request.method"] != "GET" {
		t.Errorf("非敏感键被改: %v", got)
	}
	if got["nested.token"] != "***" {
		t.Errorf("group 内 token 未掩码: %v", got)
	}
}

func TestFieldMaskerRecursivelyMasksStructuredValues(t *testing.T) {
	m := FieldMasker{}
	in := []slog.Attr{
		slog.Any("fields", Fields{
			"user": map[string]any{
				"credentials": []any{
					map[string]any{"authorization": "Bearer secret"},
					Fields{"nested_token": "token-value"},
				},
			},
			"roles": []string{"admin", "viewer"},
		}),
		slog.Any("valuer", nestedLogValue{}),
	}

	out := m.Mask(context.Background(), in)
	fields := out[0].Value.Any().(Fields)
	user := fields["user"].(map[string]any)
	credentials := user["credentials"].([]any)
	if got := credentials[0].(map[string]any)["authorization"]; got != "***" {
		t.Errorf("map/[]any 内 authorization = %v, want ***", got)
	}
	if got := credentials[1].(Fields)["nested_token"]; got != "***" {
		t.Errorf("Fields 内 nested_token = %v, want ***", got)
	}
	roles := fields["roles"].([]string)
	if len(roles) != 2 || roles[0] != "admin" || roles[1] != "viewer" {
		t.Errorf("[]string 非敏感值被改: %v", roles)
	}

	valuerGroup := out[1].Value.Group()
	profile := valuerGroup[0].Value.Any().(map[string]any)
	if got := profile["credentials"].(Fields)["password"]; got != "***" {
		t.Errorf("LogValuer 内 password = %v, want ***", got)
	}
}

func TestDefaultSensitiveKeysReturnsCopy(t *testing.T) {
	keys := DefaultSensitiveKeys()
	keys[0] = "not-sensitive"

	out := (FieldMasker{}).Mask(context.Background(), []slog.Attr{
		slog.String("password", "secret"),
	})
	if got := out[0].Value.String(); got != "***" {
		t.Errorf("外部修改默认键副本后 password = %q, want ***", got)
	}
}

// EventKeepSampler 单元测试。测试输入是采样判定数据，不是生产事件注册
// （AGENTS.md 规则 1 约束生产埋点；此处仅为黑盒判定样例）。

func TestEventKeepSamplerPrefixesAlwaysKeep(t *testing.T) {
	s := EventKeepSampler{
		KeepPrefixes: []string{"order.", "runtime.", "auth.", "config.", "health."},
		Fallback:     ResultKeepSampler{Ratio: 0},
	}
	for _, name := range []string{
		"order.payment.succeeded",
		"runtime.panic.occurred",
		"auth.session.denied",
		"config.change.recorded",
		"health.check.completed",
	} {
		attrs := []slog.Attr{
			slog.String(string(KeyEventName), name),
			slog.String(string(KeyAppResult), string(ResultSuccess)),
		}
		if !s.Sample(context.Background(), attrs) {
			t.Errorf("event.name %s 命中前缀应恒保留", name)
		}
	}
}

func TestEventKeepSamplerFallbackDelegation(t *testing.T) {
	s := EventKeepSampler{
		KeepPrefixes: []string{"order."},
		Fallback:     ResultKeepSampler{Ratio: 0},
	}
	access := func(result Result) []slog.Attr {
		return []slog.Attr{
			slog.String(string(KeyEventName), "http.request.completed"),
			slog.String(string(KeyAppResult), string(result)),
		}
	}
	if s.Sample(context.Background(), access(ResultSuccess)) {
		t.Error("access success 应委托 Fallback 且被丢弃（Ratio=0）")
	}
	if !s.Sample(context.Background(), access(ResultFailed)) {
		t.Error("access failed 应委托 Fallback 且被强制保留")
	}
}

func TestEventKeepSamplerNilFallback(t *testing.T) {
	s := EventKeepSampler{KeepPrefixes: []string{"order."}}
	attrs := []slog.Attr{
		slog.String(string(KeyEventName), "http.request.completed"),
		slog.String(string(KeyAppResult), string(ResultSuccess)),
	}
	if !s.Sample(context.Background(), attrs) {
		t.Error("Fallback=nil 时未命中事件应恒保留")
	}
}

func TestEventKeepSamplerMissingEventName(t *testing.T) {
	s := EventKeepSampler{
		KeepPrefixes: []string{"order."},
		Fallback:     ResultKeepSampler{Ratio: 0},
	}
	attrs := []slog.Attr{slog.String(string(KeyAppResult), string(ResultSuccess))}
	if s.Sample(context.Background(), attrs) {
		t.Error("event.name 缺失应走 Fallback（Ratio=0 → 丢弃）")
	}
}

func TestEventKeepSamplerLegacyTypePrefixCompatibility(t *testing.T) {
	s := EventKeepSampler{
		KeepPrefixes: []string{"business."},
		Fallback:     ResultKeepSampler{Ratio: 0},
	}
	attrs := []slog.Attr{
		slog.String(string(KeyEventName), "order.payment.succeeded"),
		slog.String(string(KeyAppResult), string(ResultSuccess)),
	}
	if !s.SampleEvent(context.Background(), EventBusiness, attrs) {
		t.Error("旧 business. 配置应按 EventType 兼容保留 BusinessEvent")
	}
	if s.SampleEvent(context.Background(), EventAccess, attrs) {
		t.Error("旧 business. 配置不应保留 AccessEvent")
	}
}

func TestEventTypeKeepSampler(t *testing.T) {
	s := NewEventTypeKeepSampler(
		[]EventType{EventBusiness, EventError, EventSecurity, EventAudit, EventProbe},
		ResultKeepSampler{Ratio: 0},
	)
	attrs := []slog.Attr{slog.String(string(KeyAppResult), string(ResultSuccess))}
	if !s.SampleEvent(context.Background(), EventBusiness, attrs) {
		t.Error("BusinessEvent 应按 EventType 恒保留")
	}
	if s.SampleEvent(context.Background(), EventAccess, attrs) {
		t.Error("成功 AccessEvent 应交给 Fallback 并被丢弃")
	}
	failed := []slog.Attr{slog.String(string(KeyAppResult), string(ResultFailed))}
	if !s.SampleEvent(context.Background(), EventAccess, failed) {
		t.Error("失败 AccessEvent 应由 ResultKeepSampler 强制保留")
	}
}

func TestNewResultKeepSampler(t *testing.T) {
	if s := NewResultKeepSampler(1); s == nil || s.Ratio != 1 {
		t.Errorf("NewResultKeepSampler(1) = %+v, want Ratio=1", s)
	}
	for _, ratio := range []float64{0, -0.5, 1.5} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NewResultKeepSampler(%v) 应 panic", ratio)
				}
			}()
			NewResultKeepSampler(ratio)
		}()
	}
}

func TestNewEventKeepSampler(t *testing.T) {
	s := NewEventKeepSampler([]string{"business."}, ResultKeepSampler{Ratio: 0.5})
	if s == nil || len(s.KeepPrefixes) != 1 || s.Fallback == nil {
		t.Errorf("NewEventKeepSampler 返回异常: %+v", s)
	}
	for _, prefixes := range [][]string{nil, {}, {""}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NewEventKeepSampler(%v) 应 panic", prefixes)
				}
			}()
			NewEventKeepSampler(prefixes, nil)
		}()
	}
}
