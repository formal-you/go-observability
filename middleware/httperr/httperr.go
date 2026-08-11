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
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/log"
)

// Projector 错误响应投影签名（框架无关）：返回 HTTP 状态码与响应体。
// errresp / recover / nethttp / kratos 的 ResponseProjector 配置统一使用本类型。
type Projector func(err error, requestID string) (int, any)

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
// validation→VALIDATION_ERROR、business→业务码（空则 BIZ_ERROR）、system/普通→SYS_ERROR；
// system 与普通错误返回固定消息，不透传内部错误细节。
func ResponseBody(err error, requestID string) map[string]any {
	body := map[string]any{}
	appErr, ok := asAppError(err)
	if ok {
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
	return errs.NewSystem(typ, fmt.Sprint(recovered),
		errs.WithStack(errs.CaptureStack()),
		errs.WithSource(errs.CaptureSource(1)),
	)
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
