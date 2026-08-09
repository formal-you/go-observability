package log

// errs→Event 投影（B2 定稿）：把 errs 错误体系映射为 log 事件结构体。
//
// 职责边界：本文件只做「errs.AppError → BusinessEvent / ErrorEvent」的字段投影，
// 不负责写出、采样、脱敏与告警判定。EventName 必须由调用方从 types.go 常量注册表
// 传入（遵守 AGENTS.md 规则 1，禁止手写事件名字符串）；TraceID / SpanID 由调用方
// 从 span context 提取后填入 EventMetadata，本文件不自动派生，避免事件名与链路关联漂移。

import (
	"github.com/formal-you/go-observability/errs"
)

// businessEventFromError 把 errs.AppError 投影为 BusinessEvent（B2 定稿的 business 面）。
// 适用于 KindValidation / KindBusiness：ErrorType 取 err.ErrorType()，BusinessCode 取
// err.ErrCode()（validation 为空时保持空串，由 Attrs 零值省略规则不输出），
// BusinessMessage 取 err.Error()，Source 取 err.Source()（errs.Source 转 log.Source），
// Result 固定 failed。级别缺省按 B3 定稿的 LevelOf 规则表推导，调用方已设置的
// md.Level 不被覆盖（B3 Q6：显式 Level 优先，规则表只做缺省）。
func businessEventFromError(eventName EventName, err errs.AppError, md EventMetadata) BusinessEvent {
	if md.Level == "" {
		md.Level = LevelOf(err)
	}
	return BusinessEvent{
		EventMetadata: md,
		Data: BusinessPayload{
			EventName:       eventName,
			ErrorType:       string(err.ErrorType()),
			BusinessCode:    string(err.ErrCode()),
			BusinessMessage: err.Error(),
			Source:          sourceOf(err),
			Result:          ResultFailed,
		},
	}
}

// errorEventFromError 把 errs.AppError 投影为 ErrorEvent（B2 定稿的 error 面）。
// 适用于 KindSystem：ErrorType 取 err.ErrorType()，ErrorMessage 取 err.Error()，
// Operation 取 err.ErrCode()（定稿：SystemError 可选业务码落 mall.operation，可为空），
// Retryable / RetryCount / UpstreamService / Source 仅在具体类型为 errs.SystemError 时
// 提取，其余保持零值；StackTrace 仅当 errs.StackRule(err.ErrorType()) 判定为 StackMust
// 时写入 err.Stack()（B2 Stack 规则表），其余类别留空以控制体积；Result 固定 error。
// 级别缺省按 B3 定稿的 LevelOf 规则表推导，调用方已设置的 md.Level 不被覆盖
// （B3 Q6：显式 Level 优先，规则表只做缺省）。
func errorEventFromError(eventName EventName, err errs.AppError, md EventMetadata) ErrorEvent {
	if md.Level == "" {
		md.Level = LevelOf(err)
	}
	ev := ErrorEvent{
		EventMetadata: md,
		Data: ErrorPayload{
			EventName:    eventName,
			ErrorType:    string(err.ErrorType()),
			ErrorMessage: err.Error(),
			Operation:    string(err.ErrCode()),
			Source:       sourceOf(err),
			Result:       ResultError,
		},
	}
	if se, ok := err.(errs.SystemError); ok {
		ev.Data.Retryable = se.Retryable()
		ev.Data.RetryCount = se.Retries()
		ev.Data.UpstreamService = se.Upstream()
		if errs.StackRule(err.ErrorType()) == errs.StackMust {
			ev.Data.StackTrace = se.Stack()
		}
	}
	return ev
}

// EventFromError 按 errs.AppError 的 Kind 分派投影：KindValidation / KindBusiness →
// BusinessEvent，KindSystem → ErrorEvent；未知 Kind 与 errs 零值兜底一致按系统错误处理。
// EventName 必须由调用方从 types.go 常量注册表传入，本函数不自动派生事件名（AGENTS.md
// 规则 1）；TraceID / SpanID 由调用方从 span context 提取后填入 md，本函数不自动关联链路。
func EventFromError(eventName EventName, err errs.AppError, md EventMetadata) EventPayload {
	switch err.Kind() {
	case errs.KindValidation, errs.KindBusiness:
		return businessEventFromError(eventName, err, md)
	case errs.KindSystem:
		return errorEventFromError(eventName, err, md)
	default:
		// 未知 Kind：与 errs 基类零值兜底（appError.Kind() 返回 KindSystem）保持一致，
		// 按系统错误事件投影，避免静默丢失错误信息。
		return errorEventFromError(eventName, err, md)
	}
}

// LevelOf 推导 errs.AppError 的缺省日志级别（B3 定稿规则表，单一真源）。
// 规则：validation / business 属预期内拒绝 → WARN（B3 Q3）；system 重试中且未耗尽 →
// WARN，重试耗尽或不可重试 → ERROR，panic（runtime.panic，不可重试）→ ERROR（B3 Q2）；
// 未知 Kind 兜底 ERROR。仅当调用方未显式设置 md.Level 时生效（B3 Q6：显式 Level 优先，
// 规则表只做缺省）；收口 middleware（recover 等经 EventFromError 投影）与投影层共用本表
// （B3 Q1：定级单一真源），避免各处散落 if 判定漂移。
func LevelOf(err errs.AppError) Level {
	switch err.Kind() {
	case errs.KindValidation, errs.KindBusiness:
		return LevelWarn
	case errs.KindSystem:
		if errs.IsRetryExhausted(err) {
			return LevelError
		}
		se, ok := err.(errs.SystemError)
		if !ok || !se.Retryable() || se.RetriesExhausted() {
			return LevelError
		}
		return LevelWarn
	default:
		return LevelError
	}
}

// sourceOf 从 errs.AppError 提取代码位置并转换为 log.Source。
// 具体类型为 errs.BizError / errs.SystemError 时取 Source()，否则返回零值 Source{}，
// 由 Attrs 的零值省略规则决定是否输出 code.* 字段。
func sourceOf(err errs.AppError) Source {
	switch e := err.(type) {
	case errs.BizError:
		return toSource(e.Source())
	case errs.SystemError:
		return toSource(e.Source())
	}
	return Source{}
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
