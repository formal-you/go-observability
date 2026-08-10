// Package errresp 提供 Gin 统一错误收口中间件。
//
// 职责：在链尾读取 c.Errors，按 errs.Kind 映射 HTTP 状态码与统一响应体，
// 经 log.EventFromError 投影错误事件后写出，避免 handler 各自维护响应格式与
// 字段映射；handler 只需调用 Abort 挂载错误，不自行决定状态码与响应体。
//
// 与 recovermw 的边界：本中间件只处理显式挂载到 c.Errors 的错误；recover 捕获
// panic 后直接渲染 500 并中止（不写 c.Errors）。两者组合时，panic 会跳过
// 本中间件 c.Next() 之后的代码，因此一个请求最多一个错误出口。
//
// 依赖方向：本包只依赖根 log 包、errs 与 go.opentelemetry.io/otel/trace
// （AGENTS.md 允许 middleware 层使用 OTel）；不依赖任何业务层。
package errresp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/errs"
)

// Config 中间件配置。
type Config struct {
	// Logger 必填：写出错误事件的 Logger；为空时 Middleware panic（配置错误应尽早暴露）。
	Logger *log.Logger

	// EventName 错误事件名；空值默认 log.EventNameErrorHTTPRequest。
	// 业务拒绝（validation/business）与系统错误共用该事件名，按 app.result / error.type / level 区分。
	EventName log.EventName

	// GetRequestID 从请求提取 request_id：写入响应体并补充到事件 metadata；
	// 可选，未提供则不输出该字段。
	GetRequestID func(c *gin.Context) string

	// StatusForError 错误到 HTTP 状态码的映射；空值使用 defaultStatusForError。
	StatusForError func(err error) int

	// ResponseProjector 自定义错误响应体与状态码；nil 使用默认（扁平 {code,message,request_id?}）。
	// 接入方可用它注入自定义契约形状（如嵌套 error 对象），状态码与响应体一次决定。
	ResponseProjector func(err error, requestID string) (int, gin.H)
}

// Middleware 返回收口显式业务/系统错误的 Gin 中间件。
// 它在 handler 外层注册，于 c.Next() 后读取 c.Errors.Last()：
// 无错误直接放行；有错误则按 errs.Kind 映射状态码与响应体，经 log.EventFromError
// 投影并写出错误事件，再 AbortWithStatusJSON 终止请求。
func Middleware(cfg Config) gin.HandlerFunc {
	if cfg.Logger == nil {
		panic("errresp: Logger 不能为空")
	}
	eventName := cfg.EventName
	if eventName == "" {
		eventName = log.EventNameErrorHTTPRequest
	}
	statusForError := cfg.StatusForError
	if statusForError == nil {
		statusForError = defaultStatusForError
	}
	projector := cfg.ResponseProjector
	if projector == nil {
		// 默认投影组合调用方可能覆盖的 StatusForError 与缺省响应体，保证向后兼容。
		projector = func(err error, requestID string) (int, gin.H) {
			return statusForError(err), responseBody(err, requestID)
		}
	}

	return func(c *gin.Context) {
		c.Next()

		last := c.Errors.Last()
		if last == nil || last.Err == nil {
			return
		}
		err := last.Err

		md := log.EventMetadata{}
		if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
			md.TraceID = sc.TraceID().String()
			md.SpanID = sc.SpanID().String()
		}
		if cfg.GetRequestID != nil {
			md.RequestID = cfg.GetRequestID(c)
		}

		// 统一走 log.EventFromError 投影，避免中间件重复维护字段映射；
		// Level 由 EventFromError 按 Kind 推导，不在此覆盖。
		ev := log.EventFromError(eventName, err, md)
		cfg.Logger.Emit(c.Request.Context(), ev)

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

// defaultStatusForError 按 errs.Kind 映射缺省 HTTP 状态码：
// validation→400（参数/入参校验失败）、business→409（业务规则拒绝）、
// system 与普通 error→500（非预期故障，不向客户端透传内部细节）。
// 调用方可通过 Config.StatusForError 整体覆盖。
func defaultStatusForError(err error) int {
	var appErr errs.AppError
	if errors.As(err, &appErr) {
		switch appErr.Kind() {
		case errs.KindValidation:
			return http.StatusBadRequest
		case errs.KindBusiness:
			return http.StatusConflict
		case errs.KindSystem:
			return http.StatusInternalServerError
		}
	}
	return http.StatusInternalServerError
}

// responseBody 构造统一错误响应体，与 recover 中间件保持同构：{code, message, request_id?}。
// validation/business 属预期内拒绝，透传业务消息与业务码；system 与普通 error 属
// 非预期故障，返回固定消息，避免把内部错误细节泄漏给客户端（细节只进入 ErrorEvent
// 的 exception.message，供日志侧排查）。
func responseBody(err error, requestID string) gin.H {
	body := gin.H{}
	if appErr, ok := asAppError(err); ok {
		switch appErr.Kind() {
		case errs.KindValidation:
			body["code"] = "VALIDATION_ERROR"
			body["message"] = appErr.Error()
		case errs.KindBusiness:
			body["code"] = string(appErr.ErrCode())
			if body["code"] == "" {
				body["code"] = "BIZ_ERROR"
			}
			body["message"] = appErr.Error()
		default:
			body["code"] = "SYS_ERROR"
			body["message"] = "系统繁忙，请稍后重试"
		}
	} else {
		body["code"] = "SYS_ERROR"
		body["message"] = "系统繁忙，请稍后重试"
	}
	if requestID != "" {
		body["request_id"] = requestID
	}
	return body
}

// asAppError 从错误链中提取 errs.AppError；非 AppError 返回 false。
func asAppError(err error) (errs.AppError, bool) {
	var appErr errs.AppError
	if errors.As(err, &appErr) && appErr != nil {
		return appErr, true
	}
	return nil, false
}
