package log

import (
	"net"
	"time"
)

// 公共元数据与组合结构：被所有事件/载荷复用。

// EventMetadata 公共元数据。
// service/version/env/instance 由 SDK Resource 提供，不在此出现；
// trace_id/span_id 由调用方从 span context 填充（hex 字符串），归一化输出为
// trace_id/span_id 字段，Writer 层再转为 OTLP TraceId/SpanId 字节。
type EventMetadata struct {
	Timestamp time.Time
	Level     Level
	TraceID   string
	SpanID    string
	RequestID string
	LatencyMS int64
}

// Subject 标识事件关联的用户与租户。
type Subject struct {
	UserID   string
	TenantID string
}

// Actor 标识执行安全或审计动作的用户与角色。
type Actor struct {
	UserID string
	Role   string
}

// Resource 标识事件关联的领域资源。
type Resource struct {
	Type string
	ID   string
}

// Source 代码位置（semconv code.*）。
type Source struct {
	Function string
	Filepath string
	Line     int
}

// HTTPInfo 表示 access / probe 事件使用的 HTTP 请求与响应字段。
// 延迟不在此输出：由调用方写入 EventMetadata.LatencyMS（latency_ms）。
type HTTPInfo struct {
	Method     string
	Route      string
	URLPath    string
	StatusCode int
	ClientIP   net.IP
	UserAgent  string
}
