package log

import (
	"context"
	"log/slog"
)

// ResultKeepSampler 按 app.result 强制保留高价值结果，其余按 ratio 概率保留。
// ratio 落在 (0,1]；<=0 时等价于只保留高价值；>1 按 1 处理。
// 未设置 app.result 或值为 unknown 时按 ratio 采样（不强制丢弃）。
// 默认随机源并发安全；包内测试注入的随机源由注入方保证并发安全。
type ResultKeepSampler struct {
	// Ratio 非高价值事件的保留概率，默认 1（全量）。
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

func attrString(attrs []slog.Attr, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.String()
		}
	}
	return ""
}
