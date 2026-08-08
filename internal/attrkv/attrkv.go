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

// recordAttrKeys 是 LogRecord 顶层字段对应的属性键。写 OTLP 时这些键不能作为属性：
// timestamp/level 映射为 LogRecord 顶层字段，event.name 映射为 EventName 顶层字段，
// trace_id/span_id 由 ctx 的 span context 承担。
var recordAttrKeys = map[string]struct{}{
	"timestamp":  {},
	"level":      {},
	"event.name": {},
	"trace_id":   {},
	"span_id":    {},
}

// Record 组装 OTLP LogRecord 的顶层字段，并返回应作为属性的剩余 attrs。
//
// timestamp（仅 KindTime）与 level（DEBUG/INFO/WARN/ERROR）映射为 LogRecord 顶层字段，
// 缺省分别取 time.Now() 与 INFO；event.name（仅 KindString）映射为 EventName 顶层字段；
// trace_id/span_id 直接剥离——sdk/log 的 Logger.Emit 会从 ctx 的 span context 自动关联到
// LogRecord（见 go.opentelemetry.io/otel/sdk/log 的 newRecord），事件 metadata 中的这两个值
// 仅供 file/stdout 等扁平投影使用，不写入 OTLP 属性。
func Record(msg string, attrs []slog.Attr) (otelog.Record, []slog.Attr) {
	rec := otelog.Record{}
	rec.SetTimestamp(time.Now())
	rec.SetObservedTimestamp(time.Now())
	severity, text := Severity(attrs)
	rec.SetSeverity(severity)
	rec.SetSeverityText(text)
	rec.SetBody(otelog.StringValue(msg))

	rest := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		switch {
		case a.Key == "timestamp" && a.Value.Kind() == slog.KindTime:
			if t := a.Value.Time(); !t.IsZero() {
				rec.SetTimestamp(t)
			}
			continue
		case a.Key == "event.name" && a.Value.Kind() == slog.KindString:
			rec.SetEventName(a.Value.String())
			continue
		}
		if _, reserved := recordAttrKeys[a.Key]; reserved {
			continue
		}
		rest = append(rest, a)
	}
	return rec, rest
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
