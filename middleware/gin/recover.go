// ginmw：panic 收口（httperr 契约核心的 Gin 适配）。
// 捕获 handler 中的 panic，经 httperr.SystemErrorFromPanic 构造非预期系统错误
// （runtime.panic + 必记堆栈），提取 span 填充 trace/span 后作为 ErrorEvent 写出，
// 再以统一错误响应（500 SYS_ERROR）终止请求，避免 panic 外泄到 Gin/http 层。
package ginmw

import (
	"github.com/gin-gonic/gin"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

// RecoverConfig 配置 panic 收口中间件。
type RecoverConfig struct {
	// Logger 必填：写出 ErrorEvent 的 Logger；为空时 panic（配置错误应尽早暴露）。
	Logger *log.Logger

	// EventName 错误事实名；空值默认 log.EventNameRuntimePanicOccurred。
	EventName log.EventName

	// ErrorType 低基数失败类别；空值默认 errs.TypeRuntimePanic。
	ErrorType errs.ErrorType

	// GetRequestID 从请求提取 request_id 写入响应体；可选，未提供则不输出该字段。
	GetRequestID func(c *gin.Context) string

	// ResponseProjector 自定义 panic 错误响应体与状态码；nil 使用默认
	// （500 + 扁平 {code:SYS_ERROR,message,request_id?}）。接入方可用它注入自定义契约形状。
	ResponseProjector httperr.Projector

	// InputGuard 输入风险守卫：写出 ErrorEvent 后调用，返回的 Security/Audit 事件
	// 与错误事件并存（错误出口唯一）；nil 表示不补发额外事件。风险分级由接入方维护。
	InputGuard httperr.InputGuard
}

// Recover 返回收口 Gin handler panic 的中间件。
// 它 defer recover 捕获 panic：经 httperr.SystemErrorFromPanic 构造 SystemError
// （error.type + panic 值消息 + 必记堆栈 + 代码位置），从请求 context 的 span context
// 填充 trace_id/span_id，写出 ErrorEvent，再 AbortWithStatusJSON 返回统一 500 响应；
// recover 到 nil（无 panic）时直接放行，中间件自身不会把 panic 外泄。
func Recover(cfg RecoverConfig) gin.HandlerFunc {
	if cfg.Logger == nil {
		panic("ginmw: Logger 不能为空")
	}
	eventName := cfg.EventName
	if eventName == "" {
		eventName = log.EventNameRuntimePanicOccurred
	}
	errorType := cfg.ErrorType
	if errorType == "" {
		errorType = errs.TypeRuntimePanic
	}
	projector := cfg.ResponseProjector
	if projector == nil {
		projector = httperr.DefaultProjector
	}

	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			reqID := ""
			if cfg.GetRequestID != nil {
				reqID = cfg.GetRequestID(c)
			}

			md := httperr.EventMetadataFromContext(c.Request.Context())
			md.Level = log.LevelError
			if reqID != "" {
				md.RequestID = reqID
			}

			err := httperr.SystemErrorFromPanic(errorType, r)
			// 统一走 log.EventFromError 投影，避免中间件重复维护字段映射。
			ev := log.EventFromError(eventName, err, md)
			cfg.Logger.Emit(c.Request.Context(), ev)
			httperr.EmitGuardEvents(cfg.Logger, c.Request.Context(), c.Request, err, cfg.InputGuard)

			status, body := projector(err, reqID)
			c.AbortWithStatusJSON(status, body)
		}()
		c.Next()
	}
}
