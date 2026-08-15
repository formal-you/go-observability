package log

import "log/slog"

// 归一化：把 metadata + payload 合并为扁平 attrs，并过滤公共保留键。

// event 归一化内部类型：仅包内组装；最终输出为扁平 attrs，不提供 data 嵌套序列化。
type event[T EventPayload] struct {
	EventType EventType
	Metadata  EventMetadata
	Data      T
}

// requestIDPrefixLen 是从 trace_id 派生对外报障凭证的长度（12 hex，48 bit）。
const requestIDPrefixLen = 12

// requestIDFromTraceID 取 trace_id 前缀作为 request_id；trace_id 不足长度时原样返回。
func requestIDFromTraceID(traceID string) string {
	if len(traceID) < requestIDPrefixLen {
		return traceID
	}
	return traceID[:requestIDPrefixLen]
}

// reservedKeys 与公共字段 / SDK 管理的字段冲突的键：payload 输出时直接丢弃。
// timestamp、level、request_id、latency_ms 由 metadata 承担；
// trace_id、span_id 由 span context 承担；service.*、deployment.environment 由 Resource 承担。
var reservedKeys = map[string]struct{}{
	"timestamp":                   {},
	"level":                       {},
	"type":                        {},
	"request_id":                  {},
	"latency_ms":                  {},
	"trace_id":                    {},
	"span_id":                     {},
	"service.name":                {},
	"service.version":             {},
	"service.instance.id":         {},
	"deployment.environment":      {},
	"deployment.environment.name": {},
}

// extraAttrAllowed 判断 ExtraAttrs 键是否允许注入载荷：
// 空键、canonical 键与公共保留键一律忽略。
func extraAttrAllowed(key string, canonical map[string]struct{}) bool {
	if key == "" {
		return false
	}
	if _, ok := canonical[key]; ok {
		return false
	}
	_, reserved := reservedKeys[key]
	return !reserved
}

// appendExtraAttrs 追加允许注入的 ExtraAttrs；canonical 是载荷的固定字段键集合。
func appendExtraAttrs(attrs []slog.Attr, extra []slog.Attr, canonical map[string]struct{}) []slog.Attr {
	for _, attr := range extra {
		if extraAttrAllowed(attr.Key, canonical) {
			attrs = append(attrs, attr)
		}
	}
	return attrs
}

// eventAttrs 归一化：先写 metadata，再追加 payload attrs（过滤空键与保留键）。
func eventAttrs[T EventPayload](ev event[T]) []slog.Attr {
	attrs := make([]slog.Attr, 0, 16)
	if !ev.Metadata.Timestamp.IsZero() {
		attrs = append(attrs, slog.Time(string(KeyTimestamp), ev.Metadata.Timestamp))
	}
	attrs = append(attrs, slog.String(string(KeyLevel), string(ev.Metadata.Level)))
	attrs = appendString(attrs, KeyTraceID, ev.Metadata.TraceID)
	attrs = appendString(attrs, KeySpanID, ev.Metadata.SpanID)
	// request_id：显式值优先；为空且 trace_id 非空时派生 trace_id 前缀。
	requestID := ev.Metadata.RequestID
	if requestID == "" && ev.Metadata.TraceID != "" {
		requestID = requestIDFromTraceID(ev.Metadata.TraceID)
	}
	attrs = appendString(attrs, KeyRequestID, requestID)
	attrs = appendInt64(attrs, KeyLatencyMS, ev.Metadata.LatencyMS)
	for _, a := range ev.Data.Attrs() {
		if a.Key == "" {
			continue
		}
		if _, reserved := reservedKeys[a.Key]; reserved {
			continue
		}
		attrs = append(attrs, a)
	}
	return attrs
}

// 零值省略辅助：字符串/整数零值不输出；布尔不省略（false 对 retryable 等有明确语义）。
func appendString(attrs []slog.Attr, key Key, value string) []slog.Attr {
	if value == "" {
		return attrs
	}
	return append(attrs, slog.String(string(key), value))
}

func appendInt(attrs []slog.Attr, key Key, value int) []slog.Attr {
	if value == 0 {
		return attrs
	}
	return append(attrs, slog.Int(string(key), value))
}

func appendInt64(attrs []slog.Attr, key Key, value int64) []slog.Attr {
	if value == 0 {
		return attrs
	}
	return append(attrs, slog.Int64(string(key), value))
}

func appendBool(attrs []slog.Attr, key Key, value bool) []slog.Attr {
	return append(attrs, slog.Bool(string(key), value))
}

func appendSlice(attrs []slog.Attr, key Key, value []string) []slog.Attr {
	if len(value) == 0 {
		return attrs
	}
	return append(attrs, slog.Any(string(key), value))
}

func appendFields(attrs []slog.Attr, key Key, value Fields) []slog.Attr {
	if len(value) == 0 {
		return attrs
	}
	return append(attrs, slog.Any(string(key), value))
}
