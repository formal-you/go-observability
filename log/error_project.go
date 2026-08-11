package log

// errs→Event 投影：把 errs 错误体系映射为 log 事件结构体。
//
// 职责边界：本文件只做「errs.AppError → BusinessEvent / ErrorEvent」的字段投影，
// 不负责写出、采样、脱敏与告警判定。EventName 必须由调用方从 types.go 常量注册表
// 传入（遵守 AGENTS.md 规则 1，禁止手写事件名字符串）；TraceID / SpanID 由调用方
// 从 span context 提取后填入 EventMetadata，本文件不自动派生，避免事件名与链路关联漂移。

import (
	"errors"
	"reflect"

	"github.com/formal-you/go-observability/errs"
)

// businessEventFromError 把 errs.AppError 投影为 BusinessEvent。
// 适用于 KindValidation / KindBusiness：ErrorType 取 err.ErrorType()，ErrorCode 取
// err.ErrCode()（validation 为空时保持空串，由 Attrs 零值省略规则不输出），
// BusinessMessage 取 err.Error()，Source 取 err.Source()（errs.Source 转 log.Source），
// Result 固定 failed。缺省级别由 LevelOf 推导，调用方显式设置的 md.Level 不被覆盖。
func businessEventFromError(eventName EventName, err errs.AppError, md EventMetadata) BusinessEvent {
	if md.Level == "" {
		md.Level = LevelOf(err)
	}
	return BusinessEvent{
		EventMetadata: md,
		Data: BusinessPayload{
			EventName:       eventName,
			ErrorType:       string(err.ErrorType()),
			ErrorCode:       string(err.ErrCode()),
			BusinessMessage: err.Error(),
			Source:          sourceOf(err),
			Result:          ResultFailed,
		},
	}
}

// sysEventFromError 把 errs.AppError 投影为 ErrorEvent。
// 适用于 KindSystem：ErrorType 取 err.ErrorType()，ErrorMessage 取 err.Error()，
// ErrorCode 取 err.ErrCode()（SystemError 可选业务关联码落 app.error_code，可为空），
// Retryable / RetryCount / UpstreamService / Source 从错误链中的 errs.SystemError 提取，
// 支持值和指针形式；StackTrace 仅当 errs.StackRule(err.ErrorType()) 判定为 StackMust
// 时写入 err.Stack()，其余类别留空以控制体积；Result 固定 error。
// 缺省级别由 LevelOf 推导，调用方显式设置的 md.Level 不被覆盖。
func sysEventFromError(eventName EventName, err errs.AppError, md EventMetadata) ErrorEvent {
	if md.Level == "" {
		md.Level = LevelOf(err)
	}
	ev := ErrorEvent{
		EventMetadata: md,
		Data: ErrorPayload{
			EventName:    eventName,
			ErrorType:    string(err.ErrorType()),
			ErrorMessage: err.Error(),
			ErrorCode:    string(err.ErrCode()),
			Source:       sourceOf(err),
			Result:       ResultError,
		},
	}
	var se systemErrorDetails
	if errors.As(err, &se) && !isNilValue(se) {
		ev.Data.Retryable = se.Retryable()
		ev.Data.RetryCount = se.Retries()
		ev.Data.UpstreamService = se.Upstream()
		if errs.StackRule(err.ErrorType()) == errs.StackMust {
			ev.Data.StackTrace = se.Stack()
		}
	}
	return ev
}

// EventFromError 沿错误链提取 errs.AppError 并按 Kind 分派投影：KindValidation /
// KindBusiness → BusinessEvent，KindSystem → ErrorEvent；普通 error 与未知 Kind 按系统错误处理。
// EventName 必须由调用方从 types.go 常量注册表传入，本函数不自动派生事件名（AGENTS.md
// 规则 1）；TraceID / SpanID 由调用方从 span context 提取后填入 md，本函数不自动关联链路。
func EventFromError(eventName EventName, err error, md EventMetadata) EventPayload {
	appErr, ok := appErrorOf(err)
	if !ok {
		if md.Level == "" {
			md.Level = LevelError
		}
		message := ""
		if !isNilValue(err) {
			message = err.Error()
		}
		return ErrorEvent{
			EventMetadata: md,
			Data: ErrorPayload{
				EventName:    eventName,
				ErrorType:    string(errs.TypeUnknown),
				ErrorMessage: message,
				Result:       ResultError,
			},
		}
	}
	switch appErr.Kind() {
	case errs.KindValidation, errs.KindBusiness:
		return businessEventFromError(eventName, appErr, md)
	case errs.KindSystem:
		return sysEventFromError(eventName, appErr, md)
	default:
		// 未知 Kind：与 errs 基类零值兜底（appError.Kind() 返回 KindSystem）保持一致，
		// 按系统错误事件投影，避免静默丢失错误信息。
		return sysEventFromError(eventName, appErr, md)
	}
}

// LevelOf 推导错误的缺省日志级别。
// validation / business 属预期内拒绝，返回 WARN；system 重试中且未耗尽返回 WARN，
// 重试耗尽、不可重试、普通 error 或未知 Kind 返回 ERROR。
func LevelOf(err error) Level {
	appErr, ok := appErrorOf(err)
	if !ok {
		return LevelError
	}
	switch appErr.Kind() {
	case errs.KindValidation, errs.KindBusiness:
		return LevelWarn
	case errs.KindSystem:
		var se systemErrorDetails
		if !errors.As(err, &se) || isNilValue(se) || !se.Retryable() || se.RetriesExhausted() {
			return LevelError
		}
		return LevelWarn
	default:
		return LevelError
	}
}

func appErrorOf(err error) (errs.AppError, bool) {
	if isNilValue(err) {
		return nil, false
	}
	var appErr errs.AppError
	if errors.As(err, &appErr) && !isNilValue(appErr) {
		return appErr, true
	}
	return nil, false
}

// sourceOf 从 errs.AppError 提取代码位置并转换为 log.Source。
// 沿错误链从 errs.BizError / errs.SystemError 的值或指针取 Source()；未找到时返回
// 零值 Source{}，由 Attrs 的零值省略规则决定是否输出 code.* 字段。
func sourceOf(err errs.AppError) Source {
	var source interface {
		Source() errs.Source
	}
	if errors.As(err, &source) && !isNilValue(source) {
		return toSource(source.Source())
	}
	return Source{}
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

type systemErrorDetails interface {
	Retryable() bool
	Retries() int
	RetriesExhausted() bool
	Upstream() string
	Stack() string
}

// toSource 把 errs.Source 转换为 log.Source：两者字段语义一致（code.function.name /
// code.file.path / code.line.number），仅因包边界不同需要显式转换。
func toSource(s errs.Source) Source {
	return Source{
		Function: s.Function,
		Filepath: s.Filepath,
		Line:     s.Line,
	}
}
