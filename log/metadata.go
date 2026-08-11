package log

import (
	"context"
	"net"
	"time"
)

// 公共元数据与组合结构：被所有事件/载荷复用。

// EventMetadata 公共元数据。
// service/version/env/instance 由 SDK Resource 提供，不在此出现。
// 字段语义：
//
//	TraceID   跨服务关联主键，日志↔trace 跳转靠它；
//	SpanID    单服务内把日志钉到具体子 span（大单体一个请求多个 span）；
//	RequestID 对外报障凭证，显式值优先，为空且 TraceID 非空时由归一化层派生 TraceID 前缀；
//	LatencyMS 日志侧独立延迟，trace 被采样掉时仍是 log-only 查询/SLO 的唯一延迟来源。
type EventMetadata struct {
	// Timestamp 事件发生时间；OTLP 路径映射 LogRecord.Timestamp，缺省由 Writer 用 time.Now() 兜底，零值省略。
	Timestamp time.Time

	// Level 语义化级别；OTLP 路径映射 SeverityNumber/SeverityText，file/stdout 扁平列保留。
	Level Level

	// TraceID 跨进程贯穿的请求标识（32 hex）。OTLP 路径由 ctx 的 span context 自动关联
	// LogRecord.TraceId（attrkv 剥离属性，不写属性键）；file/stdout 扁平列保留。空值省略。
	TraceID string

	// SpanID 单服务内定位到具体子 span 的标识（16 hex）。OTLP 路径由 ctx 的 span context
	// 自动注入 LogRecord.SpanId（attrkv 剥离属性）；file/stdout 扁平列保留。空值省略。
	SpanID string

	// RequestID 对外暴露的报障凭证：显式值优先（如网关 X-Request-ID），为空且 TraceID 非空时
	// 由归一化层派生 TraceID 前缀（12 hex，便于后端前缀匹配回查）。空值省略。
	RequestID string

	// LatencyMS 请求/操作耗时（毫秒）。日志侧独立承载延迟：trace 可能被头部采样丢弃，日志保留时
	// latency_ms 是 log-only 查询/SLO 的唯一延迟来源，故保留不依赖 trace。零值省略。
	LatencyMS int64
}

// Subject 标识事件关联的用户与租户，输出为 user.id 与 app.tenant_id。
type Subject struct {
	UserID   string
	TenantID string
}

// Actor 标识执行安全或审计动作的用户与角色。
type Actor struct {
	UserID string
	Role   string
}

// IdentityContext 聚合可信认证上下文中的 Subject 与 Actor。
// Subject 表示事件关联主体；Actor 只用于 SecurityEvent / AuditEvent 的执行者。
type IdentityContext struct {
	Subject Subject
	Actor   Actor
}

type identityContextKey struct{}

// WithIdentityContext 把已认证的 Subject / Actor 放入 context。
// 该值应由认证或授权边界创建，不应直接信任客户端提交的事件属性。
func WithIdentityContext(ctx context.Context, identity IdentityContext) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

// IdentityContextFromContext 返回由 WithIdentityContext 保存的可信身份上下文。
func IdentityContextFromContext(ctx context.Context) (IdentityContext, bool) {
	if ctx == nil {
		return IdentityContext{}, false
	}
	identity, ok := ctx.Value(identityContextKey{}).(IdentityContext)
	return identity, ok
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
