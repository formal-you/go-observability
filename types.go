package log

import "log/slog"

// 类型化枚举与事件接口：本文件只放类型定义与常量，不含具体载荷/事件结构。

// EventType 稳定短名（粗分类，写 event 归属；修改既有值属于 schema 兼容性变更）。
type EventType string

const (
	EventAccess   EventType = "access"
	EventBusiness EventType = "business"
	EventError    EventType = "error"
	EventSecurity EventType = "security"
	EventAudit    EventType = "audit"
	EventProbe    EventType = "probe"
)

// EventName 细名：event_type.module.action，如 business.order.paid；作为 event.name 属性输出。
type EventName string

// Level 语义化级别；Writer 适配层转 slog.Level 与 OTel SeverityNumber。
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Result 跨事件类型通用业务结果；failed/error/blocked/denied 为高价值结果，采样器强制保留。
type Result string

const (
	ResultSuccess Result = "success"
	ResultFailed  Result = "failed"
	ResultError   Result = "error"
	ResultBlocked Result = "blocked"
	ResultDenied  Result = "denied"
	ResultUnknown Result = "unknown"
)

// Fields 开放扩展结构（审计前后快照等）；Masker 递归处理 map 与 []any，自定义敏感类型由调用方显式净化。
type Fields map[string]any

// EventPayload 自描述事件：EventType 提供粗分类，Attrs 提供扁平属性（键已按 semconv / mall.* 对齐）。
// Attrs 可返回 nil；空键与公共保留键由归一化逻辑过滤。
type EventPayload interface {
	EventType() EventType
	Attrs() []slog.Attr
}
