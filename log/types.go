package log

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

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

// EventName 事件名：event.name MUST use the form <domain>.<subject>.<event>。
// <event> MUST 是注册的 Event Type（稳定语义发生，唯一标识一个 Event Structure），
// 不是自由文本，也不是 Operation Lifecycle Stage；生命周期经 Span 建模，
// 除非该生命周期转换本身就是要记录的语义事件。
// EventType 已由 msg 承载，因此 event.name 首段禁止 access/business/error/security/audit/probe。
// 本文件只登记框架级事件（Event Type 注册表）；领域事件由接入方自建注册表
// （见 example/mall），用 NewEventName 或经 Validate 的常量，禁止散落手写字符串。
// OTLP 路径由 attrkv 映射到 LogRecord 的 EventName 顶层字段；file/stdout 扁平投影保留 event.name 键。
type EventName string

const (
	// EventNameHTTPRequestCompleted 注册的 Event Type：HTTP 请求处理完成（<event>=completed，生命周期转换本身即语义事件）。
	EventNameHTTPRequestCompleted EventName = "http.request.completed"
	// EventNameDatabaseQueryTimedOut 注册的 Event Type：数据库查询超时。
	EventNameDatabaseQueryTimedOut EventName = "database.query.timed_out"
	// EventNameRuntimePanicOccurred 注册的 Event Type：运行时 panic 已发生。
	EventNameRuntimePanicOccurred EventName = "runtime.panic.occurred"
	// EventNameHTTPRequestFailed 注册的 Event Type：HTTP 请求因系统或未知错误失败。
	EventNameHTTPRequestFailed EventName = "http.request.failed"
	// EventNameHTTPRequestRejected 注册的 Event Type：HTTP 请求因校验或业务规则被拒绝。
	EventNameHTTPRequestRejected EventName = "http.request.rejected"
	// EventNameRPCRequestCompleted 注册的 Event Type：RPC 请求处理完成（<event>=completed）。
	EventNameRPCRequestCompleted EventName = "rpc.request.completed"
	// EventNameRPCRequestFailed 注册的 Event Type：RPC 请求失败。
	EventNameRPCRequestFailed EventName = "rpc.request.failed"
	// EventNameInputThreatDetected 注册的 Event Type：输入威胁已被安全检测发现。
	EventNameInputThreatDetected EventName = "input.threat.detected"
	// EventNameInputAnomalyRecorded 注册的 Event Type：输入异常已进入审计记录。
	EventNameInputAnomalyRecorded EventName = "input.anomaly.recorded"

	// EventNameAccessHTTPRequest 已弃用：请使用 EventNameHTTPRequestCompleted。
	EventNameAccessHTTPRequest = EventNameHTTPRequestCompleted
	// EventNameErrorDBTimeout 已弃用：请使用 EventNameDatabaseQueryTimedOut。
	EventNameErrorDBTimeout = EventNameDatabaseQueryTimedOut
	// EventNameErrorRuntimePanic 已弃用：请使用 EventNameRuntimePanicOccurred。
	EventNameErrorRuntimePanic = EventNameRuntimePanicOccurred
	// EventNameErrorHTTPRequest 已弃用：请使用 EventNameHTTPRequestFailed 或 resolver。
	EventNameErrorHTTPRequest = EventNameHTTPRequestFailed
	// EventNameAccessRPCRequest 已弃用：请使用 EventNameRPCRequestCompleted。
	EventNameAccessRPCRequest = EventNameRPCRequestCompleted
	// EventNameErrorRPCRequest 已弃用：请使用 EventNameRPCRequestFailed。
	EventNameErrorRPCRequest = EventNameRPCRequestFailed
	// EventNameSecurityInputAnomaly 已弃用：请使用 EventNameInputThreatDetected。
	EventNameSecurityInputAnomaly = EventNameInputThreatDetected
	// EventNameAuditInputAnomaly 已弃用：请使用 EventNameInputAnomalyRecorded。
	EventNameAuditInputAnomaly = EventNameInputAnomalyRecorded
)

// NewEventName 由三段（领域.对象.事实）构造 EventName 并做构造校验。
// 段数不是 3 或段内含非法字符（仅小写字母/数字/下划线）时 panic：
// 事件名是查询/告警依据，配置错误应尽早暴露。
func NewEventName(segments ...string) EventName {
	name := EventName(strings.Join(segments, "."))
	if err := name.Validate(); err != nil {
		panic("log: invalid event name: " + err.Error())
	}
	return name
}

// EventNamePattern 是 EventName 的规范正则：<domain>.<subject>.<event>，
// 每段仅小写字母、数字或下划线。
var EventNamePattern = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+\.[a-z0-9_]+$`)

// Validate 校验 EventName 是否符合 <domain>.<subject>.<event> 正则（每段仅小写字母/数字/下划线），
// 并拒绝以 EventType 粗分类作为首段，避免与 msg 重复表达类别。
func (e EventName) Validate() error {
	if !EventNamePattern.MatchString(string(e)) {
		return fmt.Errorf("event name %q 必须符合 <domain>.<subject>.<event> 正则", string(e))
	}
	parts := strings.Split(string(e), ".")
	switch EventType(parts[0]) {
	case EventAccess, EventBusiness, EventError, EventSecurity, EventAudit, EventProbe:
		return fmt.Errorf("event name %q 不得重复 msg 粗分类 %q", string(e), parts[0])
	}
	return nil
}

// Level 语义化级别
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

// EventPayload 自描述事件：EventType 提供粗分类，Attrs 提供扁平属性（键已按 semconv / app.* 对齐）。
// Attrs 可返回 nil；空键与公共保留键由归一化逻辑过滤。
type EventPayload interface {
	EventType() EventType
	Attrs() []slog.Attr
}
