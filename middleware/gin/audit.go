package ginmw

import (
	"github.com/gin-gonic/gin"

	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

// auditPayloadKey 是 AuditPayload 在 gin context 中的键（gin.Keys 为 string 键）。
const auditPayloadKey = "_go_obs_audit_payload"

// AuditConfig 配置审计事件中间件（AuditLog）。
type AuditConfig struct {
	// Logger 必填：写出 AuditEvent 的 Logger；为空时 panic（配置错误应尽早暴露）。
	Logger *log.Logger

	// Describe 审计描述函数：AuditLog 在链尾（handler 完成后）调用，返回非 nil 即写出
	// AuditEvent（nil=不记录）。接入方在此组装动作/actor/资源/before/after/结果；
	// 在链尾调用可读取 handler 完成后的最终状态。Describe 为空时回退读取
	// SetAudit 挂载的载荷（深层代码判定场景）。
	Describe func(c *gin.Context) *log.AuditPayload

	// GetRequestID 从请求提取 request_id 补到事件 metadata；可选，未提供则不输出该字段。
	GetRequestID func(c *gin.Context) string

	// Level 缺省级别；空值默认 INFO（审计是记录而非告警，等级由接入方按需调整）。
	Level log.Level
}

// SetAudit 把审计载荷挂到 gin context，由链尾 AuditLog 中间件写出。
// 典型场景：高权限操作或敏感资源变更完成后，由 handler 或领域代码调用；
// 未注册 AuditLog 时调用无效（事件不会写出）。
func SetAudit(c *gin.Context, payload log.AuditPayload) {
	c.Set(auditPayloadKey, payload)
}

// AuditLog 返回在链尾写出 AuditEvent 的 Gin 中间件。
// 经 SetAudit 挂载载荷后，本中间件在 c.Next() 返回后读取并写出（同一 ctx，
// trace/span 自动关联）；未挂载载荷则直接放行。审计存储（防篡改等）由接入方负责。
func AuditLog(cfg AuditConfig) gin.HandlerFunc {
	if cfg.Logger == nil {
		panic("ginmw: Logger 不能为空")
	}
	level := cfg.Level
	if level == "" {
		level = log.LevelInfo
	}
	return func(c *gin.Context) {
		c.Next()
		var payload *log.AuditPayload
		if cfg.Describe != nil {
			payload = cfg.Describe(c)
		} else if v, ok := c.Get(auditPayloadKey); ok {
			if p, ok := v.(log.AuditPayload); ok {
				payload = &p
			}
		}
		if payload == nil {
			return
		}
		md := httperr.EventMetadataFromContext(c.Request.Context())
		md.Level = level
		if cfg.GetRequestID != nil {
			md.RequestID = cfg.GetRequestID(c)
		}
		cfg.Logger.Emit(c.Request.Context(), log.AuditEvent{EventMetadata: md, Data: *payload})
	}
}
