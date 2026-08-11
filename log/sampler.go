package log

import (
	"context"
	"log/slog"
	"strings"
)

// ResultKeepSampler 按 app.result 强制保留高价值结果，其余按 ratio 概率保留。
// ratio 落在 (0,1]；<=0 时等价于只保留高价值；>1 按 1 处理。
// 未设置 app.result 或值为 unknown 时按 ratio 采样（不强制丢弃）。
// 默认随机源并发安全；包内测试注入的随机源由注入方保证并发安全。
type ResultKeepSampler struct {
	// Ratio 非高价值事件的保留概率。
	// 零值（ResultKeepSampler{}）时非高价值全部丢弃：本库没有默认填充逻辑，
	// 需要全量保留请显式传 Ratio: 1。
	Ratio float64
	// randFloat 供包内测试注入 [0,1) 随机源；nil 时使用并发安全的默认随机源。
	randFloat func() float64
}

// highValueResults 采样器强制保留的 Result 集合。
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
// 适合显式选择的高流量"业务全量 + 访问采样"策略：命中 KeepPrefixes 的事件（如 business./error./
// security./audit./probe.）恒保留，未命中的高频事件（如 access.http.request）交给
// Fallback 按结果/比例采样。启用后成功业务事件不再保证有对应 AccessEvent；event.name
// 缺失或为空时按未命中处理（安全降级）。
type EventKeepSampler struct {
	// KeepPrefixes 命中任一前缀的 event.name 恒保留；空列表时全部交给 Fallback。
	KeepPrefixes []string
	// Fallback 未命中前缀事件的采样判定；nil 时未命中事件恒保留。
	Fallback Sampler
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

func attrString(attrs []slog.Attr, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.String()
		}
	}
	return ""
}
