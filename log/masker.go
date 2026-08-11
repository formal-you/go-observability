package log

import (
	"context"
	"log/slog"
	"strings"
)

// （脱敏) 操作
// FieldMasker 按键名（精确或后缀）将属性值替换为 Redact。
// 默认敏感键见 DefaultSensitiveKeys；可通过 Keys 追加。
// 对 slog.Group、LogValuer、map 与 slice 递归处理。
// 并发安全：构造后 Keys 只读使用。
type FieldMasker struct {
	// Keys 额外敏感键（完整属性键，如 "app.user_id" 或 "password"）。
	Keys []string
	// Redact 替换文本，默认 "***"。
	Redact string
}

var defaultSensitiveKeys = [...]string{
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

// DefaultSensitiveKeys 返回基础脱敏键清单的副本（密钥/凭证类；非合规全集）。
// 不含 app.user_id 等业务标识；PII 策略由接入方经 Keys 追加。
func DefaultSensitiveKeys() []string {
	return append([]string(nil), defaultSensitiveKeys[:]...)
}

// Mask 实现 Masker。
func (m FieldMasker) Mask(_ context.Context, attrs []slog.Attr) []slog.Attr {
	redact := m.Redact
	if redact == "" {
		redact = "***"
	}
	set := make(map[string]struct{}, len(defaultSensitiveKeys)+len(m.Keys))
	for _, k := range defaultSensitiveKeys {
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
	if isSensitiveKey(a.Key, set) {
		return slog.String(a.Key, redact)
	}
	return slog.Attr{Key: a.Key, Value: maskValue(a.Value, set, redact)}
}

func isSensitiveKey(key string, set map[string]struct{}) bool {
	keyLower := strings.ToLower(key)
	if _, ok := set[keyLower]; ok {
		return true
	}
	for sk := range set {
		if strings.HasSuffix(keyLower, "."+sk) || strings.HasSuffix(keyLower, "_"+sk) {
			return true
		}
	}
	return false
}

func maskValue(value slog.Value, set map[string]struct{}, redact string) slog.Value {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindGroup:
		g := value.Group()
		ng := make([]slog.Attr, len(g))
		for i, ga := range g {
			ng[i] = maskAttr(ga, set, redact)
		}
		return slog.GroupValue(ng...)
	case slog.KindAny:
		return slog.AnyValue(maskAny(value.Any(), set, redact))
	default:
		return value
	}
}

func maskAny(value any, set map[string]struct{}, redact string) any {
	switch v := value.(type) {
	case Fields:
		masked := make(Fields, len(v))
		for key, item := range v {
			masked[key] = maskMapValue(key, item, set, redact)
		}
		return masked
	case map[string]any:
		masked := make(map[string]any, len(v))
		for key, item := range v {
			masked[key] = maskMapValue(key, item, set, redact)
		}
		return masked
	case []any:
		masked := make([]any, len(v))
		for i, item := range v {
			masked[i] = maskAny(item, set, redact)
		}
		return masked
	case []string:
		return append([]string(nil), v...)
	case slog.Attr:
		return maskAttr(v, set, redact)
	case slog.Value:
		return maskValue(v, set, redact)
	case slog.LogValuer:
		return maskValue(slog.AnyValue(v), set, redact)
	default:
		return value
	}
}

func maskMapValue(key string, value any, set map[string]struct{}, redact string) any {
	if isSensitiveKey(key, set) {
		return redact
	}
	return maskAny(value, set, redact)
}
