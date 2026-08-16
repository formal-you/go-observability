// Package httperr 提供框架无关的错误响应契约核心：errs.AppError → HTTP 状态码、
// 安全 reason/message/metadata、扁平响应体与 span 元数据提取。
//
// 它是错误收口中间件（errresp / recover / nethttp / kratos）共享的实现核心，不依赖任何
// Web 框架；各框架适配包只做「框架错误挂载机制 → 本核心」的薄转换。接入方自定义框架
// 适配时可直接复用本包的 StatusForError / ClassifyError / ResponseBody / Projector。
package httperr

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
)

// Projector 错误响应投影签名（框架无关）：返回 HTTP 状态码与响应体。
// errresp / recover / nethttp / kratos 的 ResponseProjector 配置统一使用本类型。
type Projector func(err error, requestID string) (int, any)

// EventNameResolver 根据实际错误选择稳定事实名。HTTP/Gin/Kratos 错误出口使用本类型，
// 框架不提供泛化错误事件名（ADR-0018），接入方必须经 resolver 提供符合 EventNamePattern 的具体事实名。
type EventNameResolver func(err error) log.EventName

// StatusForError 按 errs.Kind 映射缺省 HTTP 状态码：validation→400（参数/入参校验失败）、
// business→409（业务规则拒绝）、system 与普通 error→500（非预期故障）。
func StatusForError(err error) int {
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

// ClassifyError 返回稳定的 reason、安全 message 与 metadata（error.type）：
// validation/business 属预期内拒绝，透传业务描述；system 与普通 error 属非预期故障，
// 返回固定文案，不透传内部细节（细节只进入错误事件供日志侧排查）。
func ClassifyError(err error) (reason, message string, metadata map[string]string) {
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

// ResponseBody 构造扁平错误响应体 {code, message, request_id?}：errresp / nethttp 的默认契约。
// validation→业务码（空则 VALIDATION_ERROR）、business→业务码（空则 BIZ_ERROR）、system→SYS_ERROR、
// 普通 error→UNKNOWN_ERROR；system 与普通错误返回固定消息，不透传内部错误细节。
// 当前处于未发布版本的开发期：校验/业务拒绝直接透出真实业务码，便于开发期直接查看结果。
func ResponseBody(err error, requestID string) map[string]any {
	body := map[string]any{}
	appErr, ok := asAppError(err)
	if ok {
		switch appErr.Kind() {
		case errs.KindValidation:
			body["code"] = string(appErr.ErrCode())
			if body["code"] == "" {
				body["code"] = "VALIDATION_ERROR"
			}
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
		body["code"] = "UNKNOWN_ERROR"
		body["message"] = "发生未知错误"
	}
	if requestID != "" {
		body["request_id"] = requestID
	}
	return body
}

// DefaultProjector 默认投影：StatusForError + ResponseBody。
func DefaultProjector(err error, requestID string) (int, any) {
	return StatusForError(err), ResponseBody(err, requestID)
}

// EventMetadataFromContext 从 ctx 的 span context 填充 TraceID/SpanID；无有效 span 时返回空 metadata。
func EventMetadataFromContext(ctx context.Context) log.EventMetadata {
	md := log.EventMetadata{}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		md.TraceID = sc.TraceID().String()
		md.SpanID = sc.SpanID().String()
	}
	return md
}

// SystemErrorFromPanic 构造非预期系统错误：错误消息为 panic 值，自动补记创建点堆栈
// （定位 panic 发生点）与代码位置（指向调用点）。各框架 panic 收口中间件共用。
func SystemErrorFromPanic(typ errs.ErrorType, recovered any) errs.SystemError {
	err, buildErr := errs.NewSystemError(errs.SystemErrorConfig{
		Type:    typ,
		Message: fmt.Sprint(recovered),
		Stack:   errs.CaptureStack(),
		Source:  errs.CaptureSource(1),
	})
	if buildErr != nil {
		panic("httperr: invalid panic error contract: " + buildErr.Error())
	}
	return err
}

// asAppError 从错误链中提取 errs.AppError；非 AppError 或 nil 返回 false。
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

func metadataOf(appErr errs.AppError) map[string]string {
	return map[string]string{"error.type": string(appErr.ErrorType())}
}

// InputSummary 非法输入摘要：接入方在 handler 里提取输入相关字段/哈希/截断文本后，
// 用 WithInputSummary 挂到 ctx，供错误收口（errresp/recover）的 InputGuard 读取。
// 设计约束：只记 app.* 摘要字段，不落原始 body（高基数且可能含 PII/凭证），
// 原始 payload 的留存与脱敏由接入方负责（配合 FieldMasker）。
type InputSummary struct {
	// Fields 与失败相关的输入字段名（app.input_field）。
	Fields []string
	// Hash 输入哈希（可选，原文不落地；app.input_hash）。
	Hash string
	// Truncated 截断前 N 字节的输入摘要（可选；app.input_truncated）。
	Truncated string
}

// Attrs 输出 InputSummary 的扁平属性（app.input_*，零值省略）。
func (s InputSummary) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 3)
	if len(s.Fields) > 0 {
		attrs = append(attrs, slog.Any(string(log.KeyAppInputField), s.Fields))
	}
	if s.Hash != "" {
		attrs = append(attrs, slog.String(string(log.KeyAppInputHash), s.Hash))
	}
	if s.Truncated != "" {
		attrs = append(attrs, slog.String(string(log.KeyAppInputTruncated), s.Truncated))
	}
	return attrs
}

// InputGuard 输入风险守卫：错误收口（errresp/recover）在写出 ErrorEvent 后调用，
// 返回的 Security/Audit 等事件由中间件用同一 logger 与 ctx 依次写出（错误事件唯一，
// 安全/审计事件可与错误事件并存）。返回 nil 表示不补发额外事件。
// 风险分级/命中规则由接入方维护，库不做自动分类；guard 仅在错误路径被调用。
type InputGuard func(ctx context.Context, r *http.Request, err error, summary InputSummary) []log.EventPayload

// WithInputSummary 把非法输入摘要挂到 ctx，供错误收口的 InputGuard 读取。
func WithInputSummary(ctx context.Context, s InputSummary) context.Context {
	return context.WithValue(ctx, inputSummaryKey{}, s)
}

// InputSummaryFromContext 从 ctx 读取输入摘要；未设置时返回零值。
func InputSummaryFromContext(ctx context.Context) InputSummary {
	s, _ := ctx.Value(inputSummaryKey{}).(InputSummary)
	return s
}

// EmitGuardEvents 调用 InputGuard 并把返回的额外事件依次写出；guard 为 nil 时不做任何事。
// 中间件在写出 ErrorEvent 之后调用，保证同一请求的错误事件唯一、安全/审计事件可并存，
// 且共享同一 ctx（trace/span 自动关联）。
func EmitGuardEvents(l *log.Logger, ctx context.Context, r *http.Request, err error, guard InputGuard) {
	if guard == nil {
		return
	}
	summary := InputSummaryFromContext(ctx)
	for _, ev := range guard(ctx, r, err, summary) {
		l.Emit(ctx, ev)
	}
}

// inputSummaryKey 是 InputSummary 在 context.Context 中的私有键类型，避免与外部键冲突。
type inputSummaryKey struct{}
