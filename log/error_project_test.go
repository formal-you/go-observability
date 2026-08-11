package log

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/formal-you/go-observability/errs"
)

// attrValue 读取归一化 attrs 中指定键的 slog.Value；缺失时直接失败。
// 供各投影测试断言扁平属性键（semconv / app.*）的实际输出。
func attrValue(t *testing.T, attrs map[string]any, key string) slog.Value {
	t.Helper()
	v, ok := attrs[key].(slog.Value)
	if !ok {
		t.Fatalf("缺少属性 %s（实际: %v）", key, keysOf(attrs))
	}
	return v
}

// eventLevel 从投影事件中读取保留/缺省的 Level（EventMetadata 提升字段）。
func eventLevel(ev EventPayload) Level {
	switch e := ev.(type) {
	case BusinessEvent:
		return e.Level
	case ErrorEvent:
		return e.Level
	}
	return ""
}

// TestBusinessEventFromValidationError 验证 validation 错误投影为 BusinessEvent：
// error.type=validation.failed、ErrorCode 空、Source 映射、Result=failed、级别缺省 WARN。
func TestBusinessEventFromValidationError(t *testing.T) {
	src := errs.Source{Function: "orderService.Create", Filepath: "service/order.go", Line: 42}
	err := errs.NewValidation("order.create: 数量必须为正整数").WithSource(src)
	ev := businessEventFromError(EventName("order.creation.rejected"), err, EventMetadata{})

	if ev.Data.EventName != EventName("order.creation.rejected") {
		t.Errorf("EventName = %q, want %q", ev.Data.EventName, EventName("order.creation.rejected"))
	}
	if ev.Data.ErrorType != "validation.failed" {
		t.Errorf("ErrorType = %q, want validation.failed", ev.Data.ErrorType)
	}
	if ev.Data.ErrorCode != "" {
		t.Errorf("ErrorCode = %q, want empty（validation 不记 ErrCode）", ev.Data.ErrorCode)
	}
	if ev.Data.BusinessMessage != "order.create: 数量必须为正整数" {
		t.Errorf("BusinessMessage = %q, want 错误消息", ev.Data.BusinessMessage)
	}
	if ev.Data.Source != (Source{Function: "orderService.Create", Filepath: "service/order.go", Line: 42}) {
		t.Errorf("Source = %+v, want 映射后的代码位置", ev.Data.Source)
	}
	if ev.Data.Result != ResultFailed {
		t.Errorf("Result = %q, want failed", ev.Data.Result)
	}
	if ev.Level != LevelWarn {
		t.Errorf("Level = %q, want WARN（validation 缺省）", ev.Level)
	}

	attrs := attrMap(ev.Attrs())
	if got := attrValue(t, attrs, "error.type").String(); got != "validation.failed" {
		t.Errorf("error.type = %q, want validation.failed", got)
	}
	if _, ok := attrs["app.error_code"]; ok {
		t.Error("app.error_code 不应输出（validation 无 ErrCode）")
	}
	if got := attrValue(t, attrs, "code.function.name").String(); got != "orderService.Create" {
		t.Errorf("code.function.name = %q, want orderService.Create", got)
	}
	if got := attrValue(t, attrs, "code.file.path").String(); got != "service/order.go" {
		t.Errorf("code.file.path = %q, want service/order.go", got)
	}
	if got := attrValue(t, attrs, "code.line.number").Int64(); got != 42 {
		t.Errorf("code.line.number = %d, want 42", got)
	}
	if got := attrValue(t, attrs, "app.result").String(); got != "failed" {
		t.Errorf("app.result = %q, want failed", got)
	}
}

// TestBusinessEventFromBusinessError 验证 business 错误投影为 BusinessEvent：
// ErrorCode 与 ErrorType（business.*）保留，调用方已设 Level 不被覆盖。
func TestBusinessEventFromBusinessError(t *testing.T) {
	const code errs.ErrorCode = "ORDER.CREATE.STOCK_INSUFFICIENT"
	err := errs.NewBusiness(code, errs.ErrorType("business.stock_insufficient"), "库存不足，当前仅剩 2 件")
	ev := businessEventFromError(EventName("order.creation.rejected"), err, EventMetadata{Level: LevelInfo})

	if ev.Data.ErrorType != "business.stock_insufficient" {
		t.Errorf("ErrorType = %q, want business.stock_insufficient", ev.Data.ErrorType)
	}
	if ev.Data.ErrorCode != string(code) {
		t.Errorf("ErrorCode = %q, want %q", ev.Data.ErrorCode, code)
	}
	if ev.Data.BusinessMessage != "库存不足，当前仅剩 2 件" {
		t.Errorf("BusinessMessage = %q, want 错误消息", ev.Data.BusinessMessage)
	}
	if ev.Level != LevelInfo {
		t.Errorf("Level = %q, want INFO（调用方已设置，不应被覆盖）", ev.Level)
	}

	attrs := attrMap(ev.Attrs())
	if got := attrValue(t, attrs, "error.type").String(); got != "business.stock_insufficient" {
		t.Errorf("error.type = %q, want business.stock_insufficient", got)
	}
	if got := attrValue(t, attrs, "app.error_code").String(); got != string(code) {
		t.Errorf("app.error_code = %q, want %q", got, code)
	}
}

// TestErrorEventFromSystemError 验证 system 错误投影为 ErrorEvent：error.type、error_code、
// retryable/retry_count、source、result=error，且 db.* 属 StackMust 写入堆栈；
// md 中的 TraceID/SpanID 原样透传，级别缺省 WARN（重试中未耗尽）。
func TestErrorEventFromSystemError(t *testing.T) {
	src := errs.Source{Function: "orderRepo.FindByID", Filepath: "internal/repo/order.go", Line: 17}
	stack := "goroutine 1 [running]:\n...首层摘录"
	err := errs.NewSystem(
		errs.TypeDBQueryTimeout,
		"mysql: context deadline exceeded",
		errs.WithRetry(2, false),
		errs.WithUpstream("order-db"),
		errs.WithStack(stack),
		errs.WithSource(src),
	)
	md := EventMetadata{
		TraceID: "0123456789abcdef0123456789abcdef",
		SpanID:  "0123456789abcdef",
	}
	ev := sysEventFromError(EventNameDatabaseQueryTimedOut, err, md)

	if ev.Data.EventName != EventNameDatabaseQueryTimedOut {
		t.Errorf("EventName = %q, want %q", ev.Data.EventName, EventNameDatabaseQueryTimedOut)
	}
	if ev.Data.ErrorType != "db.query_timeout" {
		t.Errorf("ErrorType = %q, want db.query_timeout", ev.Data.ErrorType)
	}
	if ev.Data.ErrorMessage != "mysql: context deadline exceeded" {
		t.Errorf("ErrorMessage = %q, want 错误消息", ev.Data.ErrorMessage)
	}
	if ev.Data.ErrorCode != "" {
		t.Errorf("ErrorCode = %q, want empty（未设置 ErrCode）", ev.Data.ErrorCode)
	}
	if !ev.Data.Retryable {
		t.Error("Retryable = false, want true")
	}
	if ev.Data.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", ev.Data.RetryCount)
	}
	if ev.Data.UpstreamService != "order-db" {
		t.Errorf("UpstreamService = %q, want order-db", ev.Data.UpstreamService)
	}
	if ev.Data.Source != (Source{Function: "orderRepo.FindByID", Filepath: "internal/repo/order.go", Line: 17}) {
		t.Errorf("Source = %+v, want 映射后的代码位置", ev.Data.Source)
	}
	if ev.Data.Result != ResultError {
		t.Errorf("Result = %q, want error", ev.Data.Result)
	}
	if ev.Data.StackTrace != stack {
		t.Errorf("StackTrace = %q, want 堆栈（db.* 为 StackMust 应写入）", ev.Data.StackTrace)
	}
	if ev.TraceID != md.TraceID || ev.SpanID != md.SpanID {
		t.Errorf("TraceID/SpanID 未透传：TraceID=%q SpanID=%q", ev.TraceID, ev.SpanID)
	}
	if ev.Level != LevelWarn {
		t.Errorf("Level = %q, want WARN（重试中未耗尽）", ev.Level)
	}

	attrs := attrMap(ev.Attrs())
	if got := attrValue(t, attrs, "error.type").String(); got != "db.query_timeout" {
		t.Errorf("error.type = %q, want db.query_timeout", got)
	}
	if got := attrValue(t, attrs, "app.retryable").Bool(); !got {
		t.Error("app.retryable = false, want true")
	}
	if got := attrValue(t, attrs, "app.retry_count").Int64(); got != 2 {
		t.Errorf("app.retry_count = %d, want 2", got)
	}
	if _, ok := attrs["app.error_code"]; ok {
		t.Error("app.error_code 不应输出（未设置 ErrCode）")
	}
	if got := attrValue(t, attrs, "app.result").String(); got != "error" {
		t.Errorf("app.result = %q, want error", got)
	}
}

// TestErrorEventCodeFromErrCode 验证 SystemError 可选 ErrCode 映射到 app.error_code。
func TestErrorEventCodeFromErrCode(t *testing.T) {
	err := errs.NewSystem(errs.TypeHTTPUpstream5xx, "payment-gateway: 502", errs.WithCode("ORDER.PAY.UPSTREAM_5XX"))
	ev := sysEventFromError(EventName("payment.gateway.failed"), err, EventMetadata{})
	if ev.Data.ErrorCode != "ORDER.PAY.UPSTREAM_5XX" {
		t.Errorf("ErrorCode = %q, want ORDER.PAY.UPSTREAM_5XX", ev.Data.ErrorCode)
	}
	attrs := attrMap(ev.Attrs())
	if got := attrValue(t, attrs, "app.error_code").String(); got != "ORDER.PAY.UPSTREAM_5XX" {
		t.Errorf("app.error_code = %q, want ORDER.PAY.UPSTREAM_5XX", got)
	}
	for _, legacy := range []string{"app.operation", "app.business_code"} {
		if _, ok := attrs[legacy]; ok {
			t.Errorf("不应输出旧错误码键 %s", legacy)
		}
	}
}

// TestErrorEventStackOnlyWhenMust 验证 StackTrace 仅在 StackRule==StackMust 时写入：
// stock.race 属 StackOptional，即使 err 已带堆栈也不投影。
func TestErrorEventStackOnlyWhenMust(t *testing.T) {
	err := errs.NewSystem(
		errs.TypeStockRace,
		"库存扣减冲突",
		errs.WithRetry(1, false),
		errs.WithStack("不应投影的堆栈"),
	)
	ev := sysEventFromError(EventNameDatabaseQueryTimedOut, err, EventMetadata{})
	if ev.Data.StackTrace != "" {
		t.Errorf("StackTrace = %q, want empty（stock.race 非 StackMust）", ev.Data.StackTrace)
	}
}

// TestEventFromErrorDispatchesByKind 验证 EventFromError 按 Kind 分派：
// validation / business → BusinessEvent，system → ErrorEvent。
func TestEventFromErrorDispatchesByKind(t *testing.T) {
	cases := []struct {
		name string
		err  errs.AppError
		want EventType
	}{
		{"validation", errs.NewValidation("name is required"), EventBusiness},
		{"business", errs.NewBusiness("ORDER.CREATE.STOCK_INSUFFICIENT", errs.ErrorType("business.stock_insufficient"), "库存不足"), EventBusiness},
		{"system", errs.NewSystem(errs.TypeRedisTimeout, "redis timeout"), EventError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := EventFromError(EventName("order.payment.failed"), tc.err, EventMetadata{})
			if ev.EventType() != tc.want {
				t.Errorf("EventType() = %q, want %q", ev.EventType(), tc.want)
			}
			switch tc.want {
			case EventBusiness:
				if _, ok := ev.(BusinessEvent); !ok {
					t.Errorf("类型 = %T, want BusinessEvent", ev)
				}
			case EventError:
				if _, ok := ev.(ErrorEvent); !ok {
					t.Errorf("类型 = %T, want ErrorEvent", ev)
				}
			}
		})
	}
}

// TestProjectedLevelDefaults 验证级别缺省规则（B3 定稿 LevelOf 规则表）：validation/business=WARN、
// system 重试中=WARN、耗尽=ERROR、不可重试=ERROR。
func TestProjectedLevelDefaults(t *testing.T) {
	cases := []struct {
		name string
		err  errs.AppError
		want Level
	}{
		{"validation", errs.NewValidation("v"), LevelWarn},
		{"business", errs.NewBusiness("C", errs.ErrorType("business.x"), "b"), LevelWarn},
		{"system retrying", errs.NewSystem(errs.TypeDBQueryTimeout, "t", errs.WithRetry(1, false)), LevelWarn},
		{"system exhausted", errs.NewSystem(errs.TypeMQRetryExhausted, "e", errs.WithRetry(5, true)), LevelError},
		{"system not retryable", errs.NewSystem(errs.TypeRuntimePanic, "p"), LevelError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := EventFromError(EventName("order.payment.failed"), tc.err, EventMetadata{})
			if got := eventLevel(ev); got != tc.want {
				t.Errorf("Level = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCallerLevelNotOverridden 验证调用方已设置的 md.Level 不被投影覆盖：
// business 与重试中的 system 在显式 LevelError 下保持 ERROR。
func TestCallerLevelNotOverridden(t *testing.T) {
	biz := EventFromError(EventName("order.payment.failed"),
		errs.NewBusiness("C", errs.ErrorType("business.x"), "b"),
		EventMetadata{Level: LevelError})
	if got := eventLevel(biz); got != LevelError {
		t.Errorf("business Level = %q, want ERROR（调用方已设）", got)
	}

	sys := EventFromError(EventNameDatabaseQueryTimedOut,
		errs.NewSystem(errs.TypeDBQueryTimeout, "t", errs.WithRetry(1, false)),
		EventMetadata{Level: LevelError})
	if got := eventLevel(sys); got != LevelError {
		t.Errorf("system Level = %q, want ERROR（调用方已设）", got)
	}
}

type wrappedAppError struct {
	errs.AppError
	cause error
}

func (e wrappedAppError) Error() string { return "wrapped: " + e.cause.Error() }
func (e wrappedAppError) Unwrap() error { return e.cause }

type typedNilErrorWrapper struct {
	cause error
}

func (typedNilErrorWrapper) Error() string   { return "wrapped typed-nil error" }
func (e typedNilErrorWrapper) Unwrap() error { return e.cause }

// TestErrorProjectionConcreteForms 验证错误详情可从值、指针及错误链中提取。
func TestErrorProjectionConcreteForms(t *testing.T) {
	src := errs.Source{Function: "payment.Call", Filepath: "payment/client.go", Line: 19}
	base := errs.NewSystem(
		errs.TypeHTTPUpstreamTimeout,
		"timeout",
		errs.WithRetry(3, true),
		errs.WithUpstream("payment"),
		errs.WithStack("stack"),
		errs.WithSource(src),
	)
	wrapper := wrappedAppError{AppError: base, cause: fmt.Errorf("context: %w", &base)}

	for _, tc := range []struct {
		name string
		err  errs.AppError
	}{
		{name: "value", err: base},
		{name: "pointer", err: &base},
		{name: "wrapped pointer", err: wrapper},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev := sysEventFromError(EventNameDatabaseQueryTimedOut, tc.err, EventMetadata{})
			if !ev.Data.Retryable || ev.Data.RetryCount != 3 {
				t.Errorf("retry = %v/%d, want true/3", ev.Data.Retryable, ev.Data.RetryCount)
			}
			if ev.Data.UpstreamService != "payment" || ev.Data.StackTrace != "stack" {
				t.Errorf("upstream/stack = %q/%q, want payment/stack", ev.Data.UpstreamService, ev.Data.StackTrace)
			}
			if ev.Data.Source != (Source{Function: src.Function, Filepath: src.Filepath, Line: src.Line}) {
				t.Errorf("Source = %+v, want %+v", ev.Data.Source, src)
			}
		})
	}
}

// TestBusinessProjectionConcreteForms 验证 BizError 的值、指针及错误链均保留 source。
func TestBusinessProjectionConcreteForms(t *testing.T) {
	src := errs.Source{Function: "order.Validate", Filepath: "order/validate.go", Line: 8}
	base := errs.NewBusiness("ORDER.INVALID", errs.ErrorType("business.invalid"), "invalid").WithSource(src)
	wrapper := wrappedAppError{AppError: base, cause: fmt.Errorf("context: %w", &base)}

	for _, tc := range []struct {
		name string
		err  errs.AppError
	}{
		{name: "value", err: base},
		{name: "pointer", err: &base},
		{name: "wrapped pointer", err: wrapper},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev := businessEventFromError(EventName("order.payment.failed"), tc.err, EventMetadata{})
			if ev.Data.Source != (Source{Function: src.Function, Filepath: src.Filepath, Line: src.Line}) {
				t.Errorf("Source = %+v, want %+v", ev.Data.Source, src)
			}
		})
	}
}

// TestEventFromWrappedErrors 验证公开入口接受标准 %w 包装并提取内部错误详情。
func TestEventFromWrappedErrors(t *testing.T) {
	systemSource := errs.Source{Function: "payment.Call", Filepath: "payment/client.go", Line: 20}
	system := errs.NewSystem(
		errs.TypeHTTPUpstreamTimeout,
		"timeout",
		errs.WithRetry(4, true),
		errs.WithUpstream("payment"),
		errs.WithStack("wrapped stack"),
		errs.WithSource(systemSource),
	)
	systemEvent, ok := EventFromError(
		EventNameDatabaseQueryTimedOut,
		fmt.Errorf("checkout failed: %w", &system),
		EventMetadata{},
	).(ErrorEvent)
	if !ok {
		t.Fatal("wrapped SystemError did not project to ErrorEvent")
	}
	if !systemEvent.Data.Retryable || systemEvent.Data.RetryCount != 4 || systemEvent.Data.UpstreamService != "payment" {
		t.Errorf("wrapped system details = %+v", systemEvent.Data)
	}
	if systemEvent.Data.StackTrace != "wrapped stack" || systemEvent.Data.Source.Line != 20 {
		t.Errorf("wrapped system stack/source = %+v", systemEvent.Data)
	}
	if got := LevelOf(fmt.Errorf("wrapped: %w", &system)); got != LevelError {
		t.Errorf("LevelOf(wrapped exhausted) = %q, want ERROR", got)
	}

	businessSource := errs.Source{Function: "order.Validate", Filepath: "order/validate.go", Line: 9}
	business := errs.NewBusiness("ORDER.INVALID", errs.ErrorType("business.invalid"), "invalid").WithSource(businessSource)
	businessEvent, ok := EventFromError(
		EventName("order.payment.failed"),
		fmt.Errorf("validation failed: %w", &business),
		EventMetadata{},
	).(BusinessEvent)
	if !ok {
		t.Fatal("wrapped BizError did not project to BusinessEvent")
	}
	if businessEvent.Data.Source.Line != 9 || businessEvent.Data.ErrorCode != "ORDER.INVALID" {
		t.Errorf("wrapped business details = %+v", businessEvent.Data)
	}
}

// TestEventFromPlainError 验证非 AppError 仍按系统错误安全投影。
func TestEventFromPlainError(t *testing.T) {
	ev, ok := EventFromError(EventNameDatabaseQueryTimedOut, fmt.Errorf("plain"), EventMetadata{}).(ErrorEvent)
	if !ok {
		t.Fatal("plain error did not project to ErrorEvent")
	}
	if ev.Data.ErrorType != string(errs.TypeUnknown) || ev.Data.ErrorMessage != "plain" || ev.Data.Result != ResultError || ev.Level != LevelError {
		t.Errorf("plain error projection = %+v", ev)
	}
}

func TestErrorProjectionTypedNilFallsBackSafely(t *testing.T) {
	var systemErr *errs.SystemError
	for _, tc := range []struct {
		name        string
		err         error
		wantMessage string
	}{
		{name: "direct", err: systemErr},
		{name: "wrapped", err: typedNilErrorWrapper{cause: systemErr}, wantMessage: "wrapped typed-nil error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev, ok := EventFromError(EventNameDatabaseQueryTimedOut, tc.err, EventMetadata{}).(ErrorEvent)
			if !ok {
				t.Fatalf("typed-nil error projected as %T, want ErrorEvent", ev)
			}
			if ev.Data.ErrorType != string(errs.TypeUnknown) || ev.Data.ErrorMessage != tc.wantMessage || ev.Level != LevelError {
				t.Errorf("typed-nil projection = %+v, want unknown/%q/ERROR", ev, tc.wantMessage)
			}
			if got := LevelOf(tc.err); got != LevelError {
				t.Errorf("LevelOf(typed-nil) = %q, want ERROR", got)
			}
		})
	}
}

// TestBusinessPayloadErrorFieldsOmittedWhenEmpty 验证 ErrorType / Source 零值省略：
// 普通成功业务事件未设置 error.type 与 code.* 时，Attrs 不输出这些键。
func TestBusinessPayloadErrorFieldsOmittedWhenEmpty(t *testing.T) {
	ev := BusinessEvent{
		EventMetadata: EventMetadata{Level: LevelInfo},
		Data: BusinessPayload{
			EventName: EventName("order.payment.succeeded"),
			Result:    ResultSuccess,
		},
	}
	attrs := attrMap(ev.Attrs())
	for _, k := range []string{"error.type", "code.function.name", "code.file.path", "code.line.number"} {
		if _, ok := attrs[k]; ok {
			t.Errorf("零值字段不应输出：%s", k)
		}
	}
}

// TestLevelOfRuleTable 直接验证 LevelOf 规则表（B3 定稿单一真源，收口 middleware 与
// 投影层共用）：validation/business=WARN；system 重试中=WARN、耗尽/不可重试=ERROR；
// 未知 Kind 兜底 ERROR。
func TestLevelOfRuleTable(t *testing.T) {
	cases := []struct {
		name string
		err  errs.AppError
		want Level
	}{
		{"validation", errs.NewValidation("v"), LevelWarn},
		{"business", errs.NewBusiness("C", errs.ErrorType("business.x"), "b"), LevelWarn},
		{"system retrying", errs.NewSystem(errs.TypeDBQueryTimeout, "t", errs.WithRetry(1, false)), LevelWarn},
		{"system exhausted", errs.NewSystem(errs.TypeMQRetryExhausted, "e", errs.WithRetry(5, true)), LevelError},
		{"system not retryable", errs.NewSystem(errs.TypeRuntimePanic, "p"), LevelError},
		{"unknown kind falls back to error", levelOfUnknownKind{}, LevelError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LevelOf(tc.err); got != tc.want {
				t.Errorf("LevelOf() = %q, want %q", got, tc.want)
			}
		})
	}
}

// levelOfUnknownKind 测试桩：Kind 返回未知值，验证 LevelOf 默认兜底 ERROR，
// 与 errs 基类零值兜底（appError.Kind() 返回 KindSystem）语义一致。
type levelOfUnknownKind struct{}

func (levelOfUnknownKind) Kind() errs.ErrorKind      { return errs.ErrorKind("unknown") }
func (levelOfUnknownKind) Error() string             { return "unknown kind" }
func (levelOfUnknownKind) ErrorType() errs.ErrorType { return errs.TypeRuntimePanic }
func (levelOfUnknownKind) ErrCode() errs.ErrorCode   { return "" }
func (levelOfUnknownKind) Unwrap() error             { return nil }

func TestErrorEventStackRespectsStackPolicyOverride(t *testing.T) {
	// 使用方把 db. 覆盖为 none：即使显式 WithStack，事件也不渲染 StackTrace。
	errs.SetStackPolicy(map[string]errs.StackPolicy{"db.": errs.StackNone})
	t.Cleanup(func() { errs.SetStackPolicy(nil) })

	err := errs.NewSystem(errs.TypeDBQueryTimeout, "query timeout after 5s", errs.WithStack("不应投影的堆栈"))
	ev := sysEventFromError(EventNameDatabaseQueryTimedOut, err, EventMetadata{})
	if ev.Data.StackTrace != "" {
		t.Errorf("StackTrace = %q, want empty（db. 覆盖为 none）", ev.Data.StackTrace)
	}
}
