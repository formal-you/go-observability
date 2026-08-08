// Package attrkv 提供 slog.Attr 到 OTel log KeyValue 的转换（内部共享，非公开 API）。
package attrkv

import (
	"fmt"
	"log/slog"
	"time"

	otelog "go.opentelemetry.io/otel/log"
)

// ToKeyValues 把扁平 attrs 转换为 OTel log KeyValue 列表。
func ToKeyValues(attrs []slog.Attr) []otelog.KeyValue {
	kvs := make([]otelog.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		kvs = append(kvs, toKeyValue(a))
	}
	return kvs
}

// Severity 从 attrs 的 level 字段映射 OTel 严重度；缺省 INFO。
func Severity(attrs []slog.Attr) (otelog.Severity, string) {
	for _, a := range attrs {
		if a.Key == "level" {
			switch a.Value.String() {
			case "DEBUG":
				return otelog.SeverityDebug, "DEBUG"
			case "INFO":
				return otelog.SeverityInfo, "INFO"
			case "WARN":
				return otelog.SeverityWarn, "WARN"
			case "ERROR":
				return otelog.SeverityError, "ERROR"
			}
		}
	}
	return otelog.SeverityInfo, "INFO"
}

func toKeyValue(a slog.Attr) otelog.KeyValue {
	v := a.Value
	switch v.Kind() {
	case slog.KindString:
		return otelog.String(a.Key, v.String())
	case slog.KindInt64:
		return otelog.Int64(a.Key, v.Int64())
	case slog.KindUint64:
		return otelog.Int64(a.Key, int64(v.Uint64()))
	case slog.KindFloat64:
		return otelog.Float64(a.Key, v.Float64())
	case slog.KindBool:
		return otelog.Bool(a.Key, v.Bool())
	case slog.KindTime:
		return otelog.String(a.Key, v.Time().Format(time.RFC3339Nano))
	case slog.KindDuration:
		return otelog.String(a.Key, v.Duration().String())
	case slog.KindGroup:
		group := v.Group()
		vals := make([]otelog.Value, 0, len(group))
		for _, ga := range group {
			vals = append(vals, toKeyValue(ga).Value)
		}
		return otelog.Slice(a.Key, vals...)
	default:
		return otelog.String(a.Key, fmt.Sprint(v.Any()))
	}
}
