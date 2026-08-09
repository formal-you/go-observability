package log

import (
	"context"
	"log/slog"
	"testing"
)

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
