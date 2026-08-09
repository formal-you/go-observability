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

// EventName 细名：固定段式 类别.模块.操作，如 access.http.request / business.order.paid。
// 常量即注册表：新增事件名必须在此登记并符合段式，避免手写字符串漂移。
// OTLP 路径由 attrkv 映射到 LogRecord 的 EventName 顶层字段；file/stdout 扁平投影保留 event.name 键。
type EventName string

const (
	// EventNameAccessHTTPRequest 访问事件：HTTP 请求完成（ginlog 中间件默认值）。
	EventNameAccessHTTPRequest EventName = "access.http.request"
	// EventNameBusinessOrderPaid 业务事件：订单支付成功。
	EventNameBusinessOrderPaid EventName = "business.order.paid"
	// EventNameErrorDBTimeout 错误事件：数据库超时。
	EventNameErrorDBTimeout EventName = "error.db.timeout"
	// EventNameErrorRuntimePanic 错误事件：运行时 panic（recover 中间件默认值）。
	EventNameErrorRuntimePanic EventName = "error.runtime.panic"
)

// NewEventName 由三段（类别.模块.操作）构造 EventName 并做构造校验。
// 段数不是 3 或段内含非法字符（仅小写字母/数字/下划线）时 panic：
// 事件名是查询/告警依据，配置错误应尽早暴露。
func NewEventName(segments ...string) EventName {
	name := EventName(strings.Join(segments, "."))
	if err := name.Validate(); err != nil {
		panic("log: invalid event name: " + err.Error())
	}
	return name
}

// Validate 校验 EventName 是否符合 类别.模块.操作 三段式（每段仅小写字母/数字/下划线）。
func (e EventName) Validate() error {
	parts := strings.Split(string(e), ".")
	if len(parts) != 3 {
		return fmt.Errorf("event name %q 必须为 类别.模块.操作 三段式", string(e))
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

// EventPayload 自描述事件：EventType 提供粗分类，Attrs 提供扁平属性（键已按 semconv / mall.* 对齐）。
// Attrs 可返回 nil；空键与公共保留键由归一化逻辑过滤。
type EventPayload interface {
	EventType() EventType
	Attrs() []slog.Attr
}
