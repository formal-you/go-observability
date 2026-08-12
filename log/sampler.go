package log

import (
	"context"
	"log/slog"
	"strings"
)

// ResultKeepSampler 按 app.result 高优先级保留高价值结果，其余按 ratio 概率保留。
// 保留属 Sampling/Retention Policy 层建议：高价值失败/异常事件 SHOULD be retained
// by the telemetry pipeline；sampling policy SHOULD prioritize or guarantee retention
// where operationally required，不编码进 Event Semantic Convention。
// 默认随机源并发安全；包内测试注入的随机源由注入方保证并发安全。
type ResultKeepSampler struct {
	// Ratio 非高价值事件的保留概率。
	// 零值（ResultKeepSampler{}）时非高价值全部丢弃：本库没有默认填充逻辑，
	// 需要全量保留请显式传 Ratio: 1。
	Ratio float64
	// randFloat 供包内测试注入 [0,1) 随机源；nil 时使用并发安全的默认随机源。
	randFloat func() float64
}

// highValueResults Sampling/Retention Policy 高优先级保留的 Result 集合（SHOULD 保留）。
var highValueResults = map[string]struct{}{
	string(ResultFailed):  {},
	string(ResultError):   {},
	string(ResultBlocked): {},
	string(ResultDenied):  {},
}

// Sample 实现 Sampler：高价值 result 恒 true；否则按 Ratio。
func (s ResultKeepSampler) Sample(_ context.Context, attrs []slog.Attr) bool {
	result := attrString(attrs, string(KeyAppResult))
	if _, ok := highValueResults[result]; ok {
		return true
	}
	r := s.Ratio
	if r <= 0 {
		return false
	}
	if r >= 1 {
		return true
	}
	rf := s.randFloat
	if rf == nil {
		rf = sampleRand
	}
	return rf() < r
}

// EventKeepSampler 按 event.name 前缀强制保留事件，其余事件委托 Fallback 判定。
// EventName 不再包含 EventType 粗分类后，新配置应优先使用 EventTypeKeepSampler；本类型
// 继续适合按领域前缀保留事件，并兼容旧的 business./error./security./audit./probe. 配置。
// 启用采样后，成功业务事件不再保证有对应 AccessEvent；event.name 缺失或为空时按未命中处理。
type EventKeepSampler struct {
	// KeepPrefixes 命中任一前缀的 event.name 恒保留；空列表时全部交给 Fallback。
	KeepPrefixes []string
	// Fallback 未命中前缀事件的采样判定；nil 时未命中事件恒保留。
	Fallback Sampler
}

// SampleEvent 实现 EventTypeSampler。除正常 event.name 前缀外，它把旧配置中的
// access./business./error./security./audit./probe. 识别为 EventType 前缀，保证
// EventName 去除类别前缀后旧采样配置不会静默丢失高价值事件。
func (s EventKeepSampler) SampleEvent(ctx context.Context, eventType EventType, attrs []slog.Attr) bool {
	legacyTypePrefix := string(eventType) + "."
	for _, prefix := range s.KeepPrefixes {
		if prefix == legacyTypePrefix {
			return true
		}
	}
	return s.Sample(ctx, attrs)
}

// Sample 实现 Sampler：event.name 命中 KeepPrefixes 任一前缀恒 true；否则委托 Fallback。
func (s EventKeepSampler) Sample(ctx context.Context, attrs []slog.Attr) bool {
	name := attrString(attrs, string(KeyEventName))
	for _, prefix := range s.KeepPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	if s.Fallback == nil {
		return true
	}
	return s.Fallback.Sample(ctx, attrs)
}

// EventTypeKeepSampler 按 EventType 粗分类高优先级保留事件（Sampling/Retention Policy 层），其余委托 Fallback。
// 推荐用它表达 business/error/security/audit/probe 全量、access 显式采样策略。
type EventTypeKeepSampler struct {
	KeepTypes []EventType
	Fallback  Sampler
}

// SampleEvent 实现 EventTypeSampler。
func (s EventTypeKeepSampler) SampleEvent(ctx context.Context, eventType EventType, attrs []slog.Attr) bool {
	for _, keepType := range s.KeepTypes {
		if eventType == keepType {
			return true
		}
	}
	if s.Fallback == nil {
		return true
	}
	return s.Fallback.Sample(ctx, attrs)
}

// Sample 在缺少 EventType 的直接调用场景委托 Fallback；Logger 会优先调用 SampleEvent。
func (s EventTypeKeepSampler) Sample(ctx context.Context, attrs []slog.Attr) bool {
	if s.Fallback == nil {
		return true
	}
	return s.Fallback.Sample(ctx, attrs)
}

// NewResultKeepSampler 构造按 app.result 采样的 ResultKeepSampler。
// ratio 必须落在 (0,1]：0 是零值陷阱（非高价值全丢），>1 是常见配置错误，均 panic。
func NewResultKeepSampler(ratio float64) *ResultKeepSampler {
	if ratio <= 0 || ratio > 1 {
		panic("log: ResultKeepSampler ratio 必须落在 (0,1]")
	}
	return &ResultKeepSampler{Ratio: ratio}
}

// NewEventKeepSampler 构造按 event.name 前缀全量保留的 EventKeepSampler。
// prefixes 必须至少含一个非空前缀（空列表会让所有事件走 Fallback，通常是配置错误）；
// fallback 可为 nil（未命中事件恒保留）。
func NewEventKeepSampler(prefixes []string, fallback Sampler) *EventKeepSampler {
	for _, prefix := range prefixes {
		if prefix != "" {
			return &EventKeepSampler{KeepPrefixes: prefixes, Fallback: fallback}
		}
	}
	panic("log: EventKeepSampler keep prefixes 至少需要一个非空前缀")
}

// NewEventTypeKeepSampler 构造按 EventType 全量保留的采样器。
// types 不能为空，且只能包含六类已登记 EventType；非法配置会 panic。
func NewEventTypeKeepSampler(types []EventType, fallback Sampler) *EventTypeKeepSampler {
	if len(types) == 0 {
		panic("log: EventTypeKeepSampler keep types 不能为空")
	}
	for _, eventType := range types {
		switch eventType {
		case EventAccess, EventBusiness, EventError, EventSecurity, EventAudit, EventProbe:
		default:
			panic("log: EventTypeKeepSampler 包含非法 EventType")
		}
	}
	return &EventTypeKeepSampler{KeepTypes: types, Fallback: fallback}
}

func attrString(attrs []slog.Attr, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.String()
		}
	}
	return ""
}
