// Package kratosmw 提供 go-observability 错误体系与 go-kratos v3 传输层的适配：
// 自定义 HTTP ErrorEncoder 与 gRPC 错误映射把 errs.AppError / kratos 原生错误映射为
// kratos 错误契约（code/reason/message/metadata，error.type 写入 metadata），以及错误
// 日志中间件把 handler 返回的错误经 log.EventFromError 投影为错误事件
// （event.name/error.type/level）。
//
// 与 kratos 默认行为的差异：非 AppError、非 kratos 原生的普通错误按 go-observability
// 语义兜底为 500/Internal + 固定文案（error.type=error.unknown），不透传内部 message；
// kratos 原生错误（*kerrors.Error）保持其 code/reason/message 契约原样透出。
package kratosmw

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	httpstatus "github.com/go-kratos/kratos/v3/transport/http/status"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"

	"github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/errs"
)

// Option 定制 kratos 适配层行为。
type Option func(*options)

type options struct {
	eventName    log.EventName
	getRequestID func(ctx context.Context) string
	statusForErr func(err error) int
	projector    func(err error, requestID string) (int, any)
}

func defaultOptions() options {
	return options{eventName: log.EventNameErrorHTTPRequest, statusForErr: defaultStatusForError}
}

// WithEventName 设置错误日志事件名；空值默认 log.EventNameErrorHTTPRequest。
func WithEventName(name log.EventName) Option {
	return func(o *options) { o.eventName = name }
}

// WithGetRequestID 从请求 context 提取 request_id 写入日志事件 metadata；可选。
func WithGetRequestID(fn func(ctx context.Context) string) Option {
	return func(o *options) { o.getRequestID = fn }
}

// WithStatusForError 自定义 errs.AppError 到 HTTP 状态码的映射；空值使用默认
// （validation→400、business→409、system→500）。kratos 原生错误不受本选项影响。
func WithStatusForError(fn func(err error) int) Option {
	return func(o *options) { o.statusForErr = fn }
}

// WithResponseProjector 自定义错误响应体与状态码；nil 使用默认 kratos 契约体
// {code,reason,message,metadata}。
func WithResponseProjector(fn func(err error, requestID string) (int, any)) Option {
	return func(o *options) { o.projector = fn }
}

// ErrorEncoder 返回 kratos HTTP 错误编码器（http.ErrorEncoder 选项）：
// kratos 原生错误（*kerrors.Error）原样透传 code/reason/message/metadata；
// errs.AppError 按 Kind 映射状态码与安全消息，error.type 写入 metadata；
// 其余普通错误兜底为 500 + 固定文案（error.type=error.unknown），不透传内部细节。
func ErrorEncoder(opts ...Option) khttp.EncodeErrorFunc {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	projector := o.projector
	if projector == nil {
		statusForError := o.statusForErr
		projector = func(err error, _ string) (int, any) {
			if se := asKratosError(err); se != nil {
				return int(se.Code), kratosErrorBody(se)
			}
			status := statusForError(err)
			return status, appErrorBody(status, err)
		}
	}
	return func(w http.ResponseWriter, r *http.Request, err error) {
		status, body := projector(err, "")
		writeJSON(w, status, body)
	}
}

// ErrorLog 返回 kratos 中间件：handler 返回错误时，把错误经 log.EventFromError 投影为
// 错误事件写入 Logger，随后把错误原样交还传输层（本中间件只负责日志，不改变响应）。
// Logger 为 nil 时直接透传（不写事件）。
func ErrorLog(logger *log.Logger, opts ...Option) middleware.Middleware {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			reply, err := next(ctx, req)
			if err == nil || logger == nil {
				return reply, err
			}
			md := log.EventMetadata{}
			if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
				md.TraceID = sc.TraceID().String()
				md.SpanID = sc.SpanID().String()
			}
			if o.getRequestID != nil {
				md.RequestID = o.getRequestID(ctx)
			}
			logger.Emit(ctx, log.EventFromError(o.eventName, err, md))
			return reply, err
		}
	}
}

// GRPCErrorMapper 返回 kratos gRPC 中间件：把 handler 返回的 errs.AppError / 普通错误
// 转换为 gRPC status error（code 由 errs.Kind 映射并经 kratos httpstatus 转 gRPC code，
// reason 与 error.type 写入 errdetails.ErrorInfo），避免 grpc-go 默认把内部 message
// 以 codes.Unknown 透传客户端。kratos 原生错误（*kerrors.Error，自带 GRPCStatus）
// 原样放行。与 ErrorLog 组合时，ErrorLog 应放在本中间件外层，以记录转换前的原始错误。
func GRPCErrorMapper(opts ...Option) middleware.Middleware {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	statusForError := o.statusForErr
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			reply, err := next(ctx, req)
			if err == nil {
				return reply, nil
			}
			if asKratosError(err) != nil {
				return reply, err
			}
			return reply, grpcStatusError(statusForError, err)
		}
	}
}

// grpcStatusError 把 errs.AppError / 普通错误转换为带 ErrorInfo detail 的 gRPC status error。
func grpcStatusError(statusForError func(err error) int, err error) error {
	code := httpstatus.ToGRPCCode(statusForError(err))
	reason, message, metadata := classifyError(err)
	st := status.New(code, message)
	if reason != "" {
		withDetail, detailErr := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Metadata: metadata})
		if detailErr == nil {
			st = withDetail
		}
	}
	return st.Err()
}

// asKratosError 沿错误链提取 kratos 原生 *kerrors.Error；非 kratos 错误返回 nil。
// 不能使用 kerrors.FromError 判断——它对任意非 kratos 错误都会合成 500 错误（永不为 nil）。
func asKratosError(err error) *kerrors.Error {
	var se *kerrors.Error
	if errors.As(err, &se) && se != nil {
		return se
	}
	return nil
}

// kratosErrorBody 构造 kratos 原生错误契约体，原样透传 code/reason/message/metadata。
func kratosErrorBody(se *kerrors.Error) map[string]any {
	metadata := se.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	return map[string]any{
		"code":     int(se.Code),
		"reason":   se.Reason,
		"message":  se.Message,
		"metadata": metadata,
	}
}

// classifyError 返回 errs.AppError / 普通错误的稳定 reason、安全 message 与 metadata
// （error.type）：validation/business 透传业务描述（预期拒绝），system 与普通错误固定
// 文案，不透传内部细节。HTTP 与 gRPC 两条出口共用。
func classifyError(err error) (reason, message string, metadata map[string]string) {
	appErr, ok := asAppError(err)
	if !ok {
		return "system_error", "internal server error", map[string]string{"error.type": string(errs.TypeUnknown)}
	}
	switch appErr.Kind() {
	case errs.KindValidation:
		return "validation_error", appErr.Error(), metadataOf(appErr)
	case errs.KindBusiness:
		reason := string(appErr.ErrCode())
		if reason == "" {
			reason = "business_error"
		}
		return reason, appErr.Error(), metadataOf(appErr)
	default:
		return "system_error", "internal server error", metadataOf(appErr)
	}
}

// appErrorBody 构造 errs.AppError（或普通错误）的 kratos HTTP 契约体。
func appErrorBody(status int, err error) map[string]any {
	reason, message, metadata := classifyError(err)
	return kratosBody(status, reason, message, metadata)
}

func kratosBody(code int, reason, message string, metadata map[string]string) map[string]any {
	return map[string]any{
		"code":     code,
		"reason":   reason,
		"message":  message,
		"metadata": metadata,
	}
}

func metadataOf(appErr errs.AppError) map[string]string {
	return map[string]string{"error.type": string(appErr.ErrorType())}
}

// defaultStatusForError 按 errs.Kind 映射缺省 HTTP 状态码：validation→400、business→409、system→500。
func defaultStatusForError(err error) int {
	appErr, ok := asAppError(err)
	if !ok {
		return http.StatusInternalServerError
	}
	switch appErr.Kind() {
	case errs.KindValidation:
		return http.StatusBadRequest
	case errs.KindBusiness:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func asAppError(err error) (errs.AppError, bool) {
	if err == nil {
		return nil, false
	}
	var appErr errs.AppError
	if errors.As(err, &appErr) && appErr != nil {
		return appErr, true
	}
	return nil, false
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
