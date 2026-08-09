package log

import (
	"log/slog"
	"testing"

	"github.com/formal-you/go-observability/errs"
)

// attrValue 读取归一化 attrs 中指定键的 slog.Value；缺失时直接失败。
// 供各投影测试断言扁平属性键（semconv / mall.*）的实际输出。
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
// error.type=validation.failed、BusinessCode 空、Source 映射、Result=failed、级别缺省 WARN。
func TestBusinessEventFromValidationError(t *testing.T) {
	src := errs.Source{Function: "orderService.Create", Filepath: "service/order.go", Line: 42}
	err := errs.NewValidation("order.create: 数量必须为正整数").WithSource(src)
	ev := businessEventFromError(EventNameBusinessOrderPaid, err, EventMetadata{})

	if ev.Data.EventName != EventNameBusinessOrderPaid {
		t.Errorf("EventName = %q, want %q", ev.Data.EventName, EventNameBusinessOrderPaid)
	}
	if ev.Data.ErrorType != "validation.failed" {
		t.Errorf("ErrorType = %q, want validation.failed", ev.Data.ErrorType)
	}
	if ev.Data.BusinessCode != "" {
		t.Errorf("BusinessCode = %q, want empty（validation 不记 ErrCode）", ev.Data.BusinessCode)
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
	if _, ok := attrs["mall.business_code"]; ok {
		t.Error("mall.business_code 不应输出（validation 无 ErrCode）")
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
	if got := attrValue(t, attrs, "mall.result").String(); got != "failed" {
		t.Errorf("mall.result = %q, want failed", got)
	}
}

// TestBusinessEventFromBusinessError 验证 business 错误投影为 BusinessEvent：
// BusinessCode 与 ErrorType（business.*）保留，调用方已设 Level 不被覆盖。
func TestBusinessEventFromBusinessError(t *testing.T) {
	const code errs.ErrorCode = "ORDER.CREATE.STOCK_INSUFFICIENT"
	err := errs.NewBusiness(code, errs.ErrorType("business.stock_insufficient"), "库存不足，当前仅剩 2 件")
	ev := businessEventFromError(EventNameBusinessOrderPaid, err, EventMetadata{Level: LevelInfo})

	if ev.Data.ErrorType != "business.stock_insufficient" {
		t.Errorf("ErrorType = %q, want business.stock_insufficient", ev.Data.ErrorType)
	}
	if ev.Data.BusinessCode != string(code) {
		t.Errorf("BusinessCode = %q, want %q", ev.Data.BusinessCode, code)
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
	if got := attrValue(t, attrs, "mall.business_code").String(); got != string(code) {
		t.Errorf("mall.business_code = %q, want %q", got, code)
	}
}

// TestErrorEventFromSystemError 验证 system 错误投影为 ErrorEvent：error.type、operation、
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
	ev := errorEventFromError(EventNameErrorDBTimeout, err, md)

	if ev.Data.EventName != EventNameErrorDBTimeout {
		t.Errorf("EventName = %q, want %q", ev.Data.EventName, EventNameErrorDBTimeout)
	}
	if ev.Data.ErrorType != "db.query_timeout" {
		t.Errorf("ErrorType = %q, want db.query_timeout", ev.Data.ErrorType)
	}
	if ev.Data.ErrorMessage != "mysql: context deadline exceeded" {
		t.Errorf("ErrorMessage = %q, want 错误消息", ev.Data.ErrorMessage)
	}
	if ev.Data.Operation != "" {
		t.Errorf("Operation = %q, want empty（未设置 ErrCode）", ev.Data.Operation)
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
	if got := attrValue(t, attrs, "mall.retryable").Bool(); !got {
		t.Error("mall.retryable = false, want true")
	}
	if got := attrValue(t, attrs, "mall.retry_count").Int64(); got != 2 {
		t.Errorf("mall.retry_count = %d, want 2", got)
	}
	if _, ok := attrs["mall.operation"]; ok {
		t.Error("mall.operation 不应输出（未设置 ErrCode）")
	}
	if got := attrValue(t, attrs, "mall.result").String(); got != "error" {
		t.Errorf("mall.result = %q, want error", got)
	}
}

// TestErrorEventOperationFromErrCode 验证 SystemError 可选 ErrCode 定稿映射到
// ErrorPayload.Operation（mall.operation），而非 mall.business_code。
func TestErrorEventOperationFromErrCode(t *testing.T) {
	err := errs.NewSystem(errs.TypeHTTPUpstream5xx, "payment-gateway: 502", errs.WithCode("ORDER.PAY.UPSTREAM_5XX"))
	ev := errorEventFromError(EventNameErrorDBTimeout, err, EventMetadata{})
	if ev.Data.Operation != "ORDER.PAY.UPSTREAM_5XX" {
		t.Errorf("Operation = %q, want ORDER.PAY.UPSTREAM_5XX", ev.Data.Operation)
	}
	attrs := attrMap(ev.Attrs())
	if got := attrValue(t, attrs, "mall.operation").String(); got != "ORDER.PAY.UPSTREAM_5XX" {
		t.Errorf("mall.operation = %q, want ORDER.PAY.UPSTREAM_5XX", got)
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
	ev := errorEventFromError(EventNameErrorDBTimeout, err, EventMetadata{})
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
			ev := EventFromError(EventNameBusinessOrderPaid, tc.err, EventMetadata{})
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
			ev := EventFromError(EventNameBusinessOrderPaid, tc.err, EventMetadata{})
			if got := eventLevel(ev); got != tc.want {
				t.Errorf("Level = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCallerLevelNotOverridden 验证调用方已设置的 md.Level 不被投影覆盖：
// business 与重试中的 system 在显式 LevelError 下保持 ERROR。
func TestCallerLevelNotOverridden(t *testing.T) {
	biz := EventFromError(EventNameBusinessOrderPaid,
		errs.NewBusiness("C", errs.ErrorType("business.x"), "b"),
		EventMetadata{Level: LevelError})
	if got := eventLevel(biz); got != LevelError {
		t.Errorf("business Level = %q, want ERROR（调用方已设）", got)
	}

	sys := EventFromError(EventNameErrorDBTimeout,
		errs.NewSystem(errs.TypeDBQueryTimeout, "t", errs.WithRetry(1, false)),
		EventMetadata{Level: LevelError})
	if got := eventLevel(sys); got != LevelError {
		t.Errorf("system Level = %q, want ERROR（调用方已设）", got)
	}
}

// TestBusinessPayloadErrorFieldsOmittedWhenEmpty 验证 ErrorType / Source 零值省略：
// 普通成功业务事件未设置 error.type 与 code.* 时，Attrs 不输出这些键。
func TestBusinessPayloadErrorFieldsOmittedWhenEmpty(t *testing.T) {
	ev := BusinessEvent{
		EventMetadata: EventMetadata{Level: LevelInfo},
		Data: BusinessPayload{
			EventName: EventNameBusinessOrderPaid,
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
