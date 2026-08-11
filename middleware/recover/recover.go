// Package recovermw 提供 Gin panic 收口中间件。
// 职责：捕获 handler 中的 panic，构造 errs.SystemError（runtime.panic + 必记堆栈），
// 提取 span 填充 trace/span 后作为 ErrorEvent 写出到 log.Logger，再以统一错误响应
// （500 SYS_ERROR）终止请求，避免 panic 外泄到 Gin/http 层。
// 依赖方向：本包只依赖根 log 包、errs 与 go.opentelemetry.io/otel/trace（AGENTS.md
// 允许 middleware 层使用 OTel）；不依赖任何业务层。
package recovermw

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
)

// Config 中间件配置。
type Config struct {
	// Logger 必填：写出 ErrorEvent 的 Logger；为空时 Middleware panic（配置错误应尽早暴露）。
	Logger *log.Logger

	// EventName 错误事件名；空值默认 log.EventNameErrorRuntimePanic。
	EventName log.EventName

	// ErrorType 低基数失败类别；空值默认 errs.TypeRuntimePanic。
	ErrorType errs.ErrorType

	// RequestID 从请求提取 request_id 写入响应体；可选，未提供则不输出该字段。
	RequestID func(c *gin.Context) string

	// ResponseProjector 自定义 panic 错误响应体与状态码；nil 使用默认
	// （500 + 扁平 {code:SYS_ERROR,message,request_id?}）。接入方可用它注入自定义契约形状。
	ResponseProjector func(err error, requestID string) (int, gin.H)
}

// defaultProjector 返回默认 panic 响应：500 + 扁平 SYS_ERROR（与 errresp 默认体同构）。
func defaultProjector(_ error, requestID string) (int, gin.H) {
	body := gin.H{
		"code":    "SYS_ERROR",
		"message": "系统繁忙，请稍后重试",
	}
	if requestID != "" {
		body["request_id"] = requestID
	}
	return http.StatusInternalServerError, body
}

// Middleware 返回收口 Gin handler panic 的中间件。
// 它 defer recover 捕获 panic：构造 errs.NewSystem（error.type + 消息 + 必记堆栈 +
// 代码位置），从请求 context 的 span context 填充 trace_id/span_id，写出 ErrorEvent，
// 再 AbortWithStatusJSON 返回统一 500 响应；recover 到 nil（无 panic）时直接放行，
// 中间件自身不会把 panic 外泄。
func Middleware(cfg Config) gin.HandlerFunc {
	if cfg.Logger == nil {
		panic("recovermw: Logger 不能为空")
	}
	eventName := cfg.EventName
	if eventName == "" {
		eventName = log.EventNameErrorRuntimePanic
	}
	errorType := cfg.ErrorType
	if errorType == "" {
		errorType = errs.TypeRuntimePanic
	}

	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			reqID := ""
			if cfg.RequestID != nil {
				reqID = cfg.RequestID(c)
			}

			md := log.EventMetadata{Level: log.LevelError}
			if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
				md.TraceID = sc.TraceID().String()
				md.SpanID = sc.SpanID().String()
			}

			err := errs.NewSystem(
				errorType,
				fmt.Sprint(r),
				errs.WithStack(errs.CaptureStack()),
				errs.WithSource(errs.CaptureSource(2)),
			)
			// 统一走 log.EventFromError 投影，避免中间件重复维护字段映射。
			ev := log.EventFromError(eventName, err, md)
			cfg.Logger.Emit(c.Request.Context(), ev)

			status, body := defaultProjector(err, reqID)
			if cfg.ResponseProjector != nil {
				status, body = cfg.ResponseProjector(err, reqID)
			}
			c.AbortWithStatusJSON(status, body)
		}()
		c.Next()
	}
}
