// Package nethttp 提供 net/http 的统一错误收口中间件，与 Gin 版 errresp/recover 对齐：
// ErrorResponse 在链尾读取 SetError 挂载的错误并渲染，Recover 捕获 panic 后渲染；
// 两者都先写出错误事件（error.http.request / error.runtime.panic），状态码与响应体
// 经 httperr 契约核心处理，可由 ResponseProjector 注入（默认扁平 {code,message,request_id?}）。
//
// Logger 为 nil 时只渲染响应体、不写事件：net/http 中间件允许应用未装配 logger 时
// 离线运行（与 errresp 的 panic-on-nil 策略不同，net/http 侧更常见无 logger 场景）。
package nethttp

import (
	"encoding/json"
	"net/http"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

// Config 中间件配置。
type Config struct {
	// Logger 写出错误事件；nil 时只渲染响应体、不写事件。
	Logger *log.Logger

	// EventName 错误事件名；空值默认 error.http.request（ErrorResponse）
	// 或 error.runtime.panic（Recover）。
	EventName log.EventName

	// GetRequestID 从请求提取 request_id：写入响应体并补充到事件 metadata；可选。
	GetRequestID func(r *http.Request) string

	// ResponseProjector 自定义错误响应体与状态码；nil 使用默认（扁平 {code,message,request_id?}）。
	ResponseProjector httperr.Projector
}

// errorWriter 捕获 handler 通过 SetError 挂载的错误，供 ErrorResponse 收口。
type errorWriter struct {
	http.ResponseWriter
	err error
}

// SetError 记录本次请求的错误。
func (w *errorWriter) SetError(err error) { w.err = err }

// errorSetter 是 ErrorResponse 注入给 handler 的窄接口。
type errorSetter interface {
	SetError(error)
}

// SetError 把错误挂载到请求链，由 ErrorResponse 统一收口（渲染响应体并写错误事件）。
// 未经过 ErrorResponse（如独立测试直接调用 handler）时按默认投影直接写出，保证行为一致。
func SetError(w http.ResponseWriter, err error) {
	if setter, ok := w.(errorSetter); ok {
		setter.SetError(err)
		return
	}
	status, body := httperr.DefaultProjector(err, "")
	writeJSON(w, status, body)
}

// ErrorResponse 返回在链尾收口显式错误的中间件：读取 SetError 挂载的错误，经
// log.EventFromError 写出错误事件后，按 ResponseProjector 渲染响应体。
// handler 只负责挂载错误，不自行决定状态码与响应体。
func ErrorResponse(cfg Config) func(http.Handler) http.Handler {
	eventName := cfg.EventName
	if eventName == "" {
		eventName = log.EventNameErrorHTTPRequest
	}
	projector := cfg.ResponseProjector
	if projector == nil {
		projector = httperr.DefaultProjector
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &errorWriter{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			if recorder.err == nil {
				return
			}
			requestID := ""
			if cfg.GetRequestID != nil {
				requestID = cfg.GetRequestID(r)
			}
			if cfg.Logger != nil {
				md := httperr.EventMetadataFromContext(r.Context())
				if requestID != "" {
					md.RequestID = requestID
				}
				cfg.Logger.Emit(r.Context(), log.EventFromError(eventName, recorder.err, md))
			}
			status, body := projector(recorder.err, requestID)
			writeJSON(w, status, body)
		})
	}
}

// Recover 返回捕获 handler panic 的中间件：构造 errs.SystemError（runtime.panic +
// 必记堆栈），写出 error.runtime.panic 事件后按 ResponseProjector 渲染响应，避免
// panic 外泄到 net/http 层。
func Recover(cfg Config) func(http.Handler) http.Handler {
	eventName := cfg.EventName
	if eventName == "" {
		eventName = log.EventNameErrorRuntimePanic
	}
	projector := cfg.ResponseProjector
	if projector == nil {
		projector = httperr.DefaultProjector
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err := httperr.SystemErrorFromPanic(errs.TypeRuntimePanic, recovered)
					requestID := ""
					if cfg.GetRequestID != nil {
						requestID = cfg.GetRequestID(r)
					}
					if cfg.Logger != nil {
						md := httperr.EventMetadataFromContext(r.Context())
						if requestID != "" {
							md.RequestID = requestID
						}
						cfg.Logger.Emit(r.Context(), log.EventFromError(eventName, err, md))
					}
					status, body := projector(err, requestID)
					writeJSON(w, status, body)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// writeJSON 写 JSON 错误响应。
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
