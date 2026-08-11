package httpmw

import (
	"net/http"

	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

// securitySetter 是 SecurityLog 注入给 handler 的窄接口。
type securitySetter interface {
	SetSecurity(log.SecurityPayload)
}

// securityWriter 记录 handler 经 SetSecurity 挂载的安全判定载荷，供 SecurityLog 收口。
type securityWriter struct {
	http.ResponseWriter
	payload *log.SecurityPayload
}

// SetSecurity 记录本次请求的安全判定载荷。
func (w *securityWriter) SetSecurity(payload log.SecurityPayload) { w.payload = &payload }

// SecurityConfig 配置安全事件中间件（SecurityLog）。
type SecurityConfig struct {
	// Logger 必填：写出 SecurityEvent 的 Logger；为空时 panic（配置错误应尽早暴露）。
	Logger *log.Logger

	// Decide 认证/授权中间件提供的判定函数：SecurityLog 在链尾调用，返回非 nil 即写出
	// SecurityEvent（nil=不记录）。判定逻辑（登录校验/鉴权/风控命中）由接入方实现，
	// 库只负责把判定结果写成事件；Decide 返回的 payload 自带 event.name、结果与
	// app.* 字段。Decide 为空时回退读取 SetSecurity 挂载的载荷（深层代码判定场景）。
	Decide func(r *http.Request) *log.SecurityPayload

	// GetRequestID 从请求提取 request_id 补到事件 metadata；可选，未提供则不输出该字段。
	GetRequestID func(r *http.Request) string

	// Level 缺省级别；空值默认 WARN（安全事件属告警性质，可被采样器按结果强制保留）。
	Level log.Level
}

// SetSecurity 把安全判定载荷挂到请求链，由 SecurityLog 统一收口写出。
// 必须在 SecurityLog 中间件包裹的 handler 内调用；未经过 SecurityLog（如独立测试
// 直接调用 handler）时调用无效（无 logger 可写出，静默忽略）。
func SetSecurity(w http.ResponseWriter, payload log.SecurityPayload) {
	if setter, ok := w.(securitySetter); ok {
		setter.SetSecurity(payload)
	}
}

// SecurityLog 返回在链尾写出 SecurityEvent 的 net/http 中间件。
// handler 或前置中间件经 SetSecurity 挂载载荷后，本中间件在 handler 返回后读取并写出
// （同一 ctx，trace/span 自动关联）；未挂载载荷则直接放行。错误路径如需按风险补发
// 安全事件，可用 errresp 的 InputGuard（见 middleware/httperr），二者不冲突。
func SecurityLog(cfg SecurityConfig) func(http.Handler) http.Handler {
	if cfg.Logger == nil {
		panic("httpmw: Logger 不能为空")
	}
	level := cfg.Level
	if level == "" {
		level = log.LevelWarn
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &securityWriter{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			var payload *log.SecurityPayload
			if cfg.Decide != nil {
				payload = cfg.Decide(r)
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
			cfg.Logger.Emit(r.Context(), log.SecurityEvent{EventMetadata: md, Data: *payload})
		})
	}
}
