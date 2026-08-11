package log

import (
	"fmt"
	"log/slog"
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

// EventName 事实名：固定三段式 领域.对象.事实，如 http.request.completed / runtime.panic.occurred。
// EventType 已由 msg 承载，因此 EventName 禁止再以 access/business/error/security/audit/probe
// 开头。本文件只登记框架级事件（中间件/通用错误）；领域事件由接入方自建注册表
// （见 example/mall），用 NewEventName 或经 Validate 的常量，禁止散落手写字符串。
// OTLP 路径由 attrkv 映射到 LogRecord 的 EventName 顶层字段；file/stdout 扁平投影保留 event.name 键。
type EventName string

const (
	// EventNameHTTPRequestCompleted 表示 HTTP 请求生命周期完成。
	EventNameHTTPRequestCompleted EventName = "http.request.completed"
	// EventNameDatabaseQueryTimedOut 表示数据库查询发生超时。
	EventNameDatabaseQueryTimedOut EventName = "database.query.timed_out"
	// EventNameRuntimePanicOccurred 表示运行时 panic 已发生。
	EventNameRuntimePanicOccurred EventName = "runtime.panic.occurred"
	// EventNameHTTPRequestFailed 表示 HTTP 请求因系统或未知错误失败。
	EventNameHTTPRequestFailed EventName = "http.request.failed"
	// EventNameHTTPRequestRejected 表示 HTTP 请求因校验或业务规则被拒绝。
	EventNameHTTPRequestRejected EventName = "http.request.rejected"
	// EventNameRPCRequestCompleted 表示 RPC 请求生命周期完成。
	EventNameRPCRequestCompleted EventName = "rpc.request.completed"
	// EventNameRPCRequestFailed 表示 RPC 请求失败。
	EventNameRPCRequestFailed EventName = "rpc.request.failed"
	// EventNameInputThreatDetected 表示输入威胁已被安全检测发现。
	EventNameInputThreatDetected EventName = "input.threat.detected"
	// EventNameInputAnomalyRecorded 表示输入异常已进入审计记录。
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

// Validate 校验 EventName 是否符合 领域.对象.事实 三段式（每段仅小写字母/数字/下划线），
// 并拒绝以 EventType 粗分类作为首段，避免与 msg 重复表达类别。
func (e EventName) Validate() error {
	parts := strings.Split(string(e), ".")
	if len(parts) != 3 {
		return fmt.Errorf("event name %q 必须为 领域.对象.事实 三段式", string(e))
	}
	for _, p := range parts {
		if p == "" {
			return fmt.Errorf("event name %q 存在空段", string(e))
		}
		for _, r := range p {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
				return fmt.Errorf("event name %q 段 %q 含非法字符（仅小写字母/数字/下划线）", string(e), p)
			}
		}
	}
	switch EventType(parts[0]) {
	case EventAccess, EventBusiness, EventError, EventSecurity, EventAudit, EventProbe:
		return fmt.Errorf("event name %q 不得重复 msg 粗分类 %q", string(e), parts[0])
	}
	return nil
}

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
