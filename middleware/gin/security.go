package ginmw

import (
	"github.com/gin-gonic/gin"

	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

// securityPayloadKey 是 SecurityPayload 在 gin context 中的键（gin.Keys 为 string 键）。
const securityPayloadKey = "_go_obs_security_payload"

// SecurityConfig 配置安全事件中间件（SecurityLog）。
type SecurityConfig struct {
	// Logger 必填：写出 SecurityEvent 的 Logger；为空时 panic（配置错误应尽早暴露）。
	Logger *log.Logger

	// GetRequestID 从请求提取 request_id 补到事件 metadata；可选，未提供则不输出该字段。
	GetRequestID func(c *gin.Context) string

	// Level 缺省级别；空值默认 WARN（安全事件属告警性质，可被采样器按结果强制保留）。
	Level log.Level
}

// SetSecurity 把安全判定载荷挂到 gin context，由链尾 SecurityLog 中间件写出。
// 典型场景：认证/鉴权/风控中间件或 handler 在做出拦截/放行判定后调用；
// 未注册 SecurityLog 时调用无效（事件不会写出）。
func SetSecurity(c *gin.Context, payload log.SecurityPayload) {
	c.Set(securityPayloadKey, payload)
}

// SecurityLog 返回在链尾写出 SecurityEvent 的 Gin 中间件。
// handler 或前置中间件经 SetSecurity 挂载载荷后，本中间件在 c.Next() 返回后读取并写出
// （同一 ctx，trace/span 自动关联）；未挂载载荷则直接放行。错误路径如需按风险补发
// 安全事件，可用 errresp 的 InputGuard（见 middleware/httperr），二者不冲突。
func SecurityLog(cfg SecurityConfig) gin.HandlerFunc {
	if cfg.Logger == nil {
		panic("ginmw: Logger 不能为空")
	}
	level := cfg.Level
	if level == "" {
		level = log.LevelWarn
	}
	return func(c *gin.Context) {
		c.Next()
		v, ok := c.Get(securityPayloadKey)
		if !ok {
			return
		}
		payload, ok := v.(log.SecurityPayload)
		if !ok {
			return
		}
		md := httperr.EventMetadataFromContext(c.Request.Context())
		md.Level = level
		if cfg.GetRequestID != nil {
			md.RequestID = cfg.GetRequestID(c)
		}
		cfg.Logger.Emit(c.Request.Context(), log.SecurityEvent{EventMetadata: md, Data: payload})
	}
}
