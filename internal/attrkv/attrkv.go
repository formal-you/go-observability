// Package attrkv 提供 slog.Attr 到 OTel log KeyValue 的转换（内部共享，非公开 API）。
package attrkv

import (
	"fmt"
	"log/slog"
	"reflect"
	"sort"
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
	return otelog.KeyValue{Key: a.Key, Value: toValue(a.Value)}
}

func toValue(v slog.Value) otelog.Value {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return otelog.StringValue(v.String())
	case slog.KindInt64:
		return otelog.Int64Value(v.Int64())
	case slog.KindUint64:
		return otelog.Int64Value(int64(v.Uint64()))
	case slog.KindFloat64:
		return otelog.Float64Value(v.Float64())
	case slog.KindBool:
		return otelog.BoolValue(v.Bool())
	case slog.KindTime:
		return otelog.StringValue(v.Time().Format(time.RFC3339Nano))
	case slog.KindDuration:
		return otelog.StringValue(v.Duration().String())
	case slog.KindGroup:
		group := v.Group()
		kvs := make([]otelog.KeyValue, 0, len(group))
		for _, ga := range group {
			kvs = append(kvs, toKeyValue(ga))
		}
		return otelog.MapValue(kvs...)
	case slog.KindAny:
		return anyValue(v.Any())
	default:
		return otelog.StringValue(fmt.Sprint(v.Any()))
	}
}

func anyValue(value any) otelog.Value {
	switch v := value.(type) {
	case nil:
		return otelog.Value{}
	case string:
		return otelog.StringValue(v)
	case bool:
		return otelog.BoolValue(v)
	case int:
		return otelog.IntValue(v)
	case int8:
		return otelog.Int64Value(int64(v))
	case int16:
		return otelog.Int64Value(int64(v))
	case int32:
		return otelog.Int64Value(int64(v))
	case int64:
		return otelog.Int64Value(v)
	case uint:
		return otelog.Int64Value(int64(v))
	case uint8:
		return otelog.Int64Value(int64(v))
	case uint16:
		return otelog.Int64Value(int64(v))
	case uint32:
		return otelog.Int64Value(int64(v))
	case uint64:
		return otelog.Int64Value(int64(v))
	case float32:
		return otelog.Float64Value(float64(v))
	case float64:
		return otelog.Float64Value(v)
	case []byte:
		return otelog.BytesValue(v)
	case time.Time:
		return otelog.StringValue(v.Format(time.RFC3339Nano))
	case time.Duration:
		return otelog.StringValue(v.String())
	case map[string]any:
		mapped, _ := stringKeyMapValue(v)
		return mapped
	case []any:
		values := make([]otelog.Value, 0, len(v))
		for _, item := range v {
			values = append(values, anyValue(item))
		}
		return otelog.SliceValue(values...)
	case []string:
		values := make([]otelog.Value, 0, len(v))
		for _, item := range v {
			values = append(values, otelog.StringValue(item))
		}
		return otelog.SliceValue(values...)
	case slog.LogValuer:
		return toValue(slog.AnyValue(v))
	default:
		if mapped, ok := stringKeyMapValue(v); ok {
			return mapped
		}
		return otelog.StringValue(fmt.Sprint(v))
	}
}

// stringKeyMapValue 把命名或未命名的 string-key map 转为 OTel MapValue。
// 反射路径用于支持 log.Fields 等公开命名 map，同时保持键排序稳定。
func stringKeyMapValue(value any) (otelog.Value, bool) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return otelog.Value{}, false
	}
	keys := rv.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	kvs := make([]otelog.KeyValue, 0, len(keys))
	for _, key := range keys {
		item := rv.MapIndex(key)
		kvs = append(kvs, otelog.KeyValue{Key: key.String(), Value: anyValue(item.Interface())})
	}
	return otelog.MapValue(kvs...), true
}
