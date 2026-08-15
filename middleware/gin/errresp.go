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

	// EventName 固定错误事实名；仅在 EventNameResolver 为空时使用。
	// 两者都为空时 panic（ADR-0018：框架不提供泛化错误事件名）。
	EventName log.EventName

	// EventNameResolver 按错误选择事实名；优先于 EventName。领域应用必须按实际错误
	// 返回注册表里的具体事实名（如系统错误码经 RegisteredEventName 反查，业务错误码经
	// 接入方注册表映射），禁止返回固定或手写字符串。
	EventNameResolver httperr.EventNameResolver

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

	// SkipEvent 命中返回 true 时跳过错误事件写出（错误事件已由接入方自行记录，
	// 例如 Application 已发布 security/business denied 领域事件），只渲染响应体；
	// nil 表示总是写出错误事件（默认，向后兼容）。
	SkipEvent func(error) bool
}

var fallbackInternalError = mustBuildFallbackError()

func mustBuildFallbackError() error {
	internal, err := errs.NewSystemError(errs.SystemErrorConfig{
		Type:    errs.TypeUnknown,
		Message: "internal error",
	})
	if err != nil {
		panic("ginmw: invalid fallback error contract: " + err.Error())
	}
	return internal
}

// ErrorResponse 返回收口显式业务/系统错误的 Gin 中间件。
// 它在 handler 外层注册，于 c.Next() 后读取 c.Errors.Last()：
// 无错误直接放行；有错误则按 errs.Kind 映射状态码与响应体，经 log.EventFromError
// 投影并写出错误事件，再 AbortWithStatusJSON 终止请求。
// ErrorResponse 返回收口显式业务/系统错误的 Gin 中间件。
// 它在 handler 外层注册，于 c.Next() 后读取 c.Errors.Last()：
// 无错误直接放行；有错误则按 errs.Kind 映射状态码与响应体，经 log.EventFromError
// 投影并写出错误事件，再 AbortWithStatusJSON 终止请求。
func ErrorResponse(cfg ErrorConfig) gin.HandlerFunc {
	if cfg.Logger == nil {
		panic("ginmw: Logger 不能为空")
	}
	eventNameResolver := resolveEventNameResolver(cfg)
	statusForError := orStatusForError(cfg.StatusForError)
	projector := orProjector(cfg.ResponseProjector, statusForError)

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

		// SkipEvent 命中时跳过事件写出（错误事件已由接入方自行记录），只渲染响应体。
		if cfg.SkipEvent == nil || !cfg.SkipEvent(err) {
			ev := log.EventFromError(eventNameResolver(err), err, md)
			cfg.Logger.Emit(c.Request.Context(), ev)
			httperr.EmitGuardEvents(cfg.Logger, c.Request.Context(), c.Request, err, cfg.InputGuard)
		}

		status, body := projector(err, md.RequestID)
		c.AbortWithStatusJSON(status, body)
	}
}

// resolveEventNameResolver 解析错误事件名来源：优先 EventNameResolver；否则用固定
// EventName（需通过 Validate）；两者都为空时 panic（ADR-0018 不提供泛化错误事件名）。
func resolveEventNameResolver(cfg ErrorConfig) httperr.EventNameResolver {
	if cfg.EventNameResolver != nil {
		return cfg.EventNameResolver
	}
	if cfg.EventName != "" {
		if err := cfg.EventName.Validate(); err != nil {
			panic("ginmw: invalid EventName: " + err.Error())
		}
		return func(error) log.EventName { return cfg.EventName }
	}
	panic("ginmw: ErrorConfig 必须提供 EventName 或 EventNameResolver")
}

func orStatusForError(f func(error) int) func(error) int {
	if f != nil {
		return f
	}
	return httperr.StatusForError
}

func orProjector(p httperr.Projector, statusForError func(error) int) httperr.Projector {
	if p != nil {
		return p
	}
	// 默认投影组合调用方可能覆盖的 StatusForError 与缺省响应体，保证向后兼容。
	return func(err error, requestID string) (int, any) {
		return statusForError(err), httperr.ResponseBody(err, requestID)
	}
}

// Abort 中止当前 Gin 链并把错误交给统一错误中间件。
// 非 nil 错误会原样写入 c.Errors，不在这里改写分类；nil 才会被替换为固定内部错误
// （errs.TypeUnknown → 500 SYS_ERROR）。固定错误在包初始化时构造并验证，因此请求路径
// 不会因库内固定契约无效而 panic。不实现 errs.AppError 的普通 error 会在 HTTP 投影
// 边界按未知系统故障兜底，handler 不应自行决定状态码。
func Abort(c *gin.Context, err error) {
	if err == nil {
		err = fallbackInternalError
	}
	_ = c.Error(err)
	c.Abort()
}
