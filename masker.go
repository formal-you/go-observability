package log

import (
	"context"
	"log/slog"
	"strings"
)

// FieldMasker 按键名（精确或后缀）将属性值替换为 Redact。
// 默认敏感键见 DefaultSensitiveKeys；可追加 Keys。
// 对 slog.KindGroup 递归处理；map/[]any 经 slog.Any 时做浅层字符串掩码。
// 并发安全：构造后 Keys 只读使用。
type FieldMasker struct {
	// Keys 额外敏感键（完整属性键，如 "app.user_id" 或 "password"）。
	Keys []string
	// Redact 替换文本，默认 "***"。
	Redact string
}

// DefaultSensitiveKeys 基础脱敏键清单（密钥/凭证类；非合规全集）。
// 不含 app.user_id 等业务标识——PII 策略由接入方经 Keys 追加。
var DefaultSensitiveKeys = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"access_token",
	"refresh_token",
	"authorization",
	"api_key",
	"apikey",
	"private_key",
}

// Mask 实现 Masker。
func (m FieldMasker) Mask(_ context.Context, attrs []slog.Attr) []slog.Attr {
	redact := m.Redact
	if redact == "" {
		redact = "***"
	}
	set := make(map[string]struct{}, len(DefaultSensitiveKeys)+len(m.Keys))
	for _, k := range DefaultSensitiveKeys {
		set[strings.ToLower(k)] = struct{}{}
	}
	for _, k := range m.Keys {
		set[strings.ToLower(k)] = struct{}{}
	}
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = maskAttr(a, set, redact)
	}
	return out
}

func maskAttr(a slog.Attr, set map[string]struct{}, redact string) slog.Attr {
	keyLower := strings.ToLower(a.Key)
	if _, ok := set[keyLower]; ok {
		return slog.String(a.Key, redact)
	}
	// 后缀匹配：*.password / authorization
	for sk := range set {
		if strings.HasSuffix(keyLower, "."+sk) || strings.HasSuffix(keyLower, "_"+sk) {
			return slog.String(a.Key, redact)
		}
	}
	switch a.Value.Kind() {
	case slog.KindGroup:
		g := a.Value.Group()
		ng := make([]slog.Attr, len(g))
		for i, ga := range g {
			ng[i] = maskAttr(ga, set, redact)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(ng...)}
	default:
		return a
	}
}
