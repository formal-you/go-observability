// Package httpmw 提供 net/http 框架体系的中间件：显式错误收口、panic 收口、
// server span 与请求指标。错误契约核心来自 middleware/httperr；本包只做 net/http 适配。
// 其他框架体系见 middleware/gin（Gin）与 middleware/grpc（gRPC）。
//
// ErrorResponse 在链尾读取 SetError 挂载的错误并渲染，Recover 捕获 panic 后渲染；
// 两者都先写出错误事件（http.request.failed / runtime.panic.occurred），状态码与响应体
// 经 httperr 契约核心处理，可由 ResponseProjector 注入（默认扁平 {code,message,request_id?}）。
//
// Logger 为 nil 时只渲染响应体、不写事件：net/http 中间件允许应用未装配 logger 时
// 离线运行（与 ginmw 的 panic-on-nil 策略不同，net/http 侧更常见无 logger 场景）。
package httpmw

import (
	"encoding/json"
	"net/http"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/middleware/httperr"
)

// ErrorConfig 配置显式错误收口与 panic 收口中间件。
type ErrorConfig struct {
	// Logger 写出错误事件；nil 时只渲染响应体、不写事件。
	Logger *log.Logger

	// EventName 固定错误事实名；ErrorResponse 仅在 EventNameResolver 为空时使用，
	// Recover 为空时默认 runtime.panic.occurred。
	EventName log.EventName

	// EventNameResolver 按错误选择事实名，仅供 ErrorResponse 使用；优先于 EventName。
	// nil 且 EventName 为空、Logger 非空时 panic：框架不提供泛化错误事件名（ADR-0018）。
	EventNameResolver httperr.EventNameResolver

	// GetRequestID 从请求提取 request_id：写入响应体并补充到事件 metadata；可选。
	GetRequestID func(r *http.Request) string

	// ResponseProjector 自定义错误响应体与状态码；nil 使用默认（扁平 {code,message,request_id?}）。
	ResponseProjector httperr.Projector

	// InputGuard 输入风险守卫：写出 ErrorEvent 后调用，返回的 Security/Audit 事件
	// 与错误事件并存（错误出口唯一）；nil 表示不补发额外事件。风险分级由接入方维护。
	InputGuard httperr.InputGuard

	// SkipEvent 命中返回 true 时跳过错误事件写出（错误事件已由接入方自行记录，
	// 例如 Application 已发布 security/business denied 领域事件），只渲染响应体；
	// nil 表示总是写出错误事件（默认，向后兼容）。
	SkipEvent func(error) bool
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
func ErrorResponse(cfg ErrorConfig) func(http.Handler) http.Handler {
	eventNameResolver := cfg.EventNameResolver
	if eventNameResolver == nil {
		if cfg.EventName != "" {
			if err := cfg.EventName.Validate(); err != nil {
				panic("httperr: invalid EventName: " + err.Error())
			}
			eventNameResolver = func(error) log.EventName { return cfg.EventName }
		} else if cfg.Logger != nil {
			// ADR-0018：框架不提供泛化错误事件名，错误事件名由接入方定义并经正则校验。
			panic("httperr: ErrorConfig 必须提供 EventName 或 EventNameResolver（Logger 非空时）")
		}
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
				// SkipEvent 命中时跳过事件写出（错误事件已由接入方自行记录），只渲染响应体。
				if cfg.SkipEvent == nil || !cfg.SkipEvent(recorder.err) {
					cfg.Logger.Emit(r.Context(), log.EventFromError(eventNameResolver(recorder.err), recorder.err, md))
					httperr.EmitGuardEvents(cfg.Logger, r.Context(), r, recorder.err, cfg.InputGuard)
				}
			}
			status, body := projector(recorder.err, requestID)
			writeJSON(w, status, body)
		})
	}
}

// Recover 返回捕获 handler panic 的中间件：经 httperr.SystemErrorFromPanic 构造非预期
// 系统错误（runtime.panic + 必记堆栈），写出 runtime.panic.occurred 事件后按
// ResponseProjector 渲染响应，避免 panic 外泄到 net/http 层。
func Recover(cfg ErrorConfig) func(http.Handler) http.Handler {
	eventName := cfg.EventName
	if eventName == "" {
		eventName = log.EventNameRuntimePanicOccurred
	}
	if err := eventName.Validate(); err != nil {
		panic("httperr: invalid Recover EventName: " + err.Error())
	}
	projector := cfg.ResponseProjector
	if projector == nil {
		projector = httperr.DefaultProjector
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err := httperr.SystemErrorFromPanic(errs.TypeInternal, recovered)
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
						httperr.EmitGuardEvents(cfg.Logger, r.Context(), r, err, cfg.InputGuard)
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
