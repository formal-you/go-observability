package httpmw

import (
	"net/http"

	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

// auditSetter 是 AuditLog 注入给 handler 的窄接口。
type auditSetter interface {
	SetAudit(log.AuditPayload)
}

// auditWriter 记录 handler 经 SetAudit 挂载的审计载荷，供 AuditLog 收口。
type auditWriter struct {
	http.ResponseWriter
	payload *log.AuditPayload
}

// SetAudit 记录本次请求的审计载荷。
func (w *auditWriter) SetAudit(payload log.AuditPayload) { w.payload = &payload }

// AuditConfig 配置审计事件中间件（AuditLog）。
type AuditConfig struct {
	// Logger 必填：写出 AuditEvent 的 Logger；为空时 panic（配置错误应尽早暴露）。
	Logger *log.Logger

	// Describe 审计描述函数：AuditLog 在链尾（handler 完成后）调用，返回非 nil 即写出
	// AuditEvent（nil=不记录）。接入方在此组装动作/actor/资源/before/after/结果；
	// 在链尾调用可读取 handler 完成后的最终状态。Describe 为空时回退读取
	// SetAudit 挂载的载荷（深层代码判定场景）。
	Describe func(r *http.Request) *log.AuditPayload

	// GetRequestID 从请求提取 request_id 补到事件 metadata；可选，未提供则不输出该字段。
	GetRequestID func(r *http.Request) string

	// Level 缺省级别；空值默认 INFO（审计是记录而非告警，等级由接入方按需调整）。
	Level log.Level
}

// SetAudit 把审计载荷挂到请求链，由 AuditLog 统一收口写出。
// 必须在 AuditLog 中间件包裹的 handler 内调用；未经过 AuditLog（如独立测试直接调用
// handler）时调用无效（无 logger 可写出，静默忽略）。
func SetAudit(w http.ResponseWriter, payload log.AuditPayload) {
	if setter, ok := w.(auditSetter); ok {
		setter.SetAudit(payload)
	}
}

// AuditLog 返回在链尾写出 AuditEvent 的 net/http 中间件。
// 经 SetAudit 挂载载荷后，本中间件在 handler 返回后读取并写出（同一 ctx，
// trace/span 自动关联）；未挂载载荷则直接放行。审计存储（防篡改等）由接入方负责。
func AuditLog(cfg AuditConfig) func(http.Handler) http.Handler {
	if cfg.Logger == nil {
		panic("httpmw: Logger 不能为空")
	}
	level := cfg.Level
	if level == "" {
		level = log.LevelInfo
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &auditWriter{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			var payload *log.AuditPayload
			if cfg.Describe != nil {
				payload = cfg.Describe(r)
			} else if recorder.payload != nil {
				payload = recorder.payload
			}
			if payload == nil {
				return
			}
			md := httperr.EventMetadataFromContext(r.Context())
			md.Level = level
			if cfg.GetRequestID != nil {
				md.RequestID = cfg.GetRequestID(r)
			}
			cfg.Logger.Emit(r.Context(), log.AuditEvent{EventMetadata: md, Data: *payload})
		})
	}
}
