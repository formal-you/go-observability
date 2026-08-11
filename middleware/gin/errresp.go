// ginmw：显式错误收口（httperr 契约核心的 Gin 适配）。
// 职责：在链尾读取 c.Errors，经 httperr 映射状态码/响应体与 span 元数据，
// 经 log.EventFromError 投影错误事件后写出；handler 只需调用 Abort 挂载错误。
//
// 与 Recover 的边界：本中间件只处理显式挂载到 c.Errors 的错误；Recover 捕获 panic
// 后直接渲染 500 并中止（不写 c.Errors）。两者组合时，panic 会跳过本中间件
// c.Next() 之后的代码，因此一个请求的错误事件唯一；经 InputGuard 注入的
// 安全/审计事件可与错误事件并存。
package ginmw

import (
	"github.com/gin-gonic/gin"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

// ErrorConfig 配置显式错误收口中间件。
type ErrorConfig struct {
	// Logger 必填：写出错误事件的 Logger；为空时 panic（配置错误应尽早暴露）。
	Logger *log.Logger

	// EventName 错误事件名；空值默认 log.EventNameErrorHTTPRequest。
	// 业务拒绝（validation/business）与系统错误共用该事件名，按 app.result / error.type / level 区分。
	EventName log.EventName

	// GetRequestID 从请求提取 request_id：写入响应体并补充到事件 metadata；
	// 可选，未提供则不输出该字段。
	GetRequestID func(c *gin.Context) string

	// StatusForError 错误到 HTTP 状态码的映射；空值使用 httperr.StatusForError。
	StatusForError func(err error) int

	// ResponseProjector 自定义错误响应体与状态码；nil 使用默认（扁平 {code,message,request_id?}）。
	// 接入方可用它注入自定义契约形状（如嵌套 error 对象），状态码与响应体一次决定。
	ResponseProjector httperr.Projector

	// InputGuard 输入风险守卫：写出 ErrorEvent 后调用，返回的 Security/Audit 事件
	// 与错误事件并存（错误出口唯一）；nil 表示不补发额外事件。风险分级由接入方维护。
	InputGuard httperr.InputGuard
}

// ErrorResponse 返回收口显式业务/系统错误的 Gin 中间件。
// 它在 handler 外层注册，于 c.Next() 后读取 c.Errors.Last()：
// 无错误直接放行；有错误则按 errs.Kind 映射状态码与响应体，经 log.EventFromError
// 投影并写出错误事件，再 AbortWithStatusJSON 终止请求。
func ErrorResponse(cfg ErrorConfig) gin.HandlerFunc {
	if cfg.Logger == nil {
		panic("ginmw: Logger 不能为空")
	}
	eventName := cfg.EventName
	if eventName == "" {
		eventName = log.EventNameErrorHTTPRequest
	}
	statusForError := cfg.StatusForError
	if statusForError == nil {
		statusForError = httperr.StatusForError
	}
	projector := cfg.ResponseProjector
	if projector == nil {
		// 默认投影组合调用方可能覆盖的 StatusForError 与缺省响应体，保证向后兼容。
		projector = func(err error, requestID string) (int, any) {
			return statusForError(err), httperr.ResponseBody(err, requestID)
		}
	}

	return func(c *gin.Context) {
		c.Next()

		last := c.Errors.Last()
		if last == nil || last.Err == nil {
			return
		}
		err := last.Err

		md := httperr.EventMetadataFromContext(c.Request.Context())
		if cfg.GetRequestID != nil {
			md.RequestID = cfg.GetRequestID(c)
		}

		// 统一走 log.EventFromError 投影，避免中间件重复维护字段映射；
		// Level 由 EventFromError 按 Kind 推导，不在此覆盖。
		ev := log.EventFromError(eventName, err, md)
		cfg.Logger.Emit(c.Request.Context(), ev)
		httperr.EmitGuardEvents(cfg.Logger, c.Request.Context(), c.Request, err, cfg.InputGuard)

		status, body := projector(err, md.RequestID)
		c.AbortWithStatusJSON(status, body)
	}
}

// Abort 中止当前 Gin 链并把错误交给统一错误中间件。
// nil 会被替换为固定内部错误（errs.TypeUnknown → 500 SYS_ERROR）；普通错误在
// HTTP 边界由中间件按 errs.Kind 映射状态码与响应体，handler 不应自行决定状态码。
func Abort(c *gin.Context, err error) {
	if err == nil {
		err = errs.NewSystem(errs.TypeUnknown, "internal error")
	}
	_ = c.Error(err)
	c.Abort()
}
