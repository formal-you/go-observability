package log

import "log/slog"

// 每个 EventType 对应一个导出的具体事件结构体（embed EventMetadata + 各自 Data）。
// 领域代码直接构造，中间件用类型断言区分事件类型；它们都实现 EventPayload，
// 可直接交给 Logger.Emit（后续 Writer 层）。

// AccessEvent 访问事件：每请求一条，记录 HTTP 请求/响应与身份字段。
type AccessEvent struct {
	EventMetadata
	Data AccessPayload
}

func (e AccessEvent) EventType() EventType { return EventAccess }

func (e AccessEvent) Attrs() []slog.Attr {
	return eventAttrs(event[AccessPayload]{EventType: EventAccess, Metadata: e.EventMetadata, Data: e.Data})
}

// BusinessEvent 业务事件：业务动作与业务拒绝。
type BusinessEvent struct {
	EventMetadata
	Data BusinessPayload
}

func (e BusinessEvent) EventType() EventType { return EventBusiness }

func (e BusinessEvent) Attrs() []slog.Attr {
	return eventAttrs(event[BusinessPayload]{EventType: EventBusiness, Metadata: e.EventMetadata, Data: e.Data})
}

// ErrorEvent 系统错误事件：错误、panic、依赖失败与重试上下文。
type ErrorEvent struct {
	EventMetadata
	Data ErrorPayload
}

func (e ErrorEvent) EventType() EventType { return EventError }

func (e ErrorEvent) Attrs() []slog.Attr {
	return eventAttrs(event[ErrorPayload]{EventType: EventError, Metadata: e.EventMetadata, Data: e.Data})
}

// SecurityEvent 安全事件：认证、鉴权、风险或访问拦截。
type SecurityEvent struct {
	EventMetadata
	Data SecurityPayload
}

func (e SecurityEvent) EventType() EventType { return EventSecurity }

func (e SecurityEvent) Attrs() []slog.Attr {
	return eventAttrs(event[SecurityPayload]{EventType: EventSecurity, Metadata: e.EventMetadata, Data: e.Data})
}

// AuditEvent 审计事件：高权限操作与敏感资源变更。
type AuditEvent struct {
	EventMetadata
	Data AuditPayload
}

func (e AuditEvent) EventType() EventType { return EventAudit }

func (e AuditEvent) Attrs() []slog.Attr {
	return eventAttrs(event[AuditPayload]{EventType: EventAudit, Metadata: e.EventMetadata, Data: e.Data})
}

// ProbeEvent 探测事件：健康、就绪、存活与启动探测。
type ProbeEvent struct {
	EventMetadata
	Data ProbePayload
}

func (e ProbeEvent) EventType() EventType { return EventProbe }

func (e ProbeEvent) Attrs() []slog.Attr {
	return eventAttrs(event[ProbePayload]{EventType: EventProbe, Metadata: e.EventMetadata, Data: e.Data})
}
