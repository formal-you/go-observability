package errs_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/formal-you/go-observability/errs"
)

// 编译期断言：BizError 与 SystemError 均实现 AppError 接口。
var (
	_ errs.AppError = errs.BizError{}
	_ errs.AppError = errs.SystemError{}
)

func TestErrorKindConstants(t *testing.T) {
	table := map[errs.ErrorKind]string{
		errs.KindValidation: "validation",
		errs.KindBusiness:   "business",
		errs.KindSystem:     "system",
	}
	for kind, want := range table {
		if got := string(kind); got != want {
			t.Errorf("ErrorKind = %q, want %q", got, want)
		}
	}
}

func TestErrorTypeConstants(t *testing.T) {
	table := map[errs.ErrorType]string{
		errs.TypeUnknown:            "UNKNOWN",
		errs.TypeCancelled:          "CANCELLED",
		errs.TypeInvalidArgument:    "INVALID_ARGUMENT",
		errs.TypeDeadlineExceeded:   "DEADLINE_EXCEEDED",
		errs.TypeNotFound:           "NOT_FOUND",
		errs.TypeAlreadyExists:      "ALREADY_EXISTS",
		errs.TypePermissionDenied:   "PERMISSION_DENIED",
		errs.TypeResourceExhausted:  "RESOURCE_EXHAUSTED",
		errs.TypeFailedPrecondition: "FAILED_PRECONDITION",
		errs.TypeAborted:            "ABORTED",
		errs.TypeOutOfRange:         "OUT_OF_RANGE",
		errs.TypeUnimplemented:      "UNIMPLEMENTED",
		errs.TypeInternal:           "INTERNAL",
		errs.TypeUnavailable:        "UNAVAILABLE",
		errs.TypeDataLoss:           "DATA_LOSS",
		errs.TypeUnauthenticated:    "UNAUTHENTICATED",
	}
	for typ, want := range table {
		if got := string(typ); got != want {
			t.Errorf("ErrorType = %q, want %q", got, want)
		}
	}
}

func TestErrorCodeValidateAndParse(t *testing.T) {
	valid := []errs.ErrorCode{"ORDER.CREATE.STOCK_INSUFFICIENT", "S1.OP_2.REASON_3"}
	for _, code := range valid {
		if err := code.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", code, err)
		}
		got, err := errs.ParseErrorCode(string(code))
		if err != nil || got != code {
			t.Errorf("ParseErrorCode(%q) = %q, %v", code, got, err)
		}
	}
	invalid := []string{"", "ORDER.CREATE", "ORDER.CREATE.FAIL.EXTRA", ".CREATE.FAIL", "ORDER..FAIL", "order.CREATE.FAIL", "ORDER.CREATE.stock", "ORDER.CREATE.BAD-REASON", " ORDER.CREATE.FAIL "}
	for _, value := range invalid {
		if err := errs.ErrorCode(value).Validate(); err == nil {
			t.Errorf("ErrorCode(%q).Validate() = nil, want error", value)
		}
		if got, err := errs.ParseErrorCode(value); err == nil || got != "" {
			t.Errorf("ParseErrorCode(%q) = %q, %v, want empty and error", value, got, err)
		}
	}
}

func TestErrorTypeValidateAndParse(t *testing.T) {
	valid := []errs.ErrorType{"UNAVAILABLE", "FAILED_PRECONDITION"}
	for _, typ := range valid {
		if err := typ.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", typ, err)
		}
		got, err := errs.ParseErrorType(string(typ))
		if err != nil || got != typ {
			t.Errorf("ParseErrorType(%q) = %q, %v", typ, got, err)
		}
	}
	invalid := []string{"", "db", "unavailable", "NOT_A_CODE", "db.connection_error", "NOT_FOUND.EXTRA", "NOT-FOUND", " not_found ", "DB.FAILED"}
	for _, value := range invalid {
		if err := errs.ErrorType(value).Validate(); err == nil {
			t.Errorf("ErrorType(%q).Validate() = nil, want error", value)
		}
		if got, err := errs.ParseErrorType(value); err == nil || got != "" {
			t.Errorf("ParseErrorType(%q) = %q, %v, want empty and error", value, got, err)
		}
	}
}

func TestStackPolicyConstants(t *testing.T) {
	table := map[errs.StackPolicy]string{
		errs.StackMust:     "must",
		errs.StackOptional: "optional",
		errs.StackNone:     "none",
	}
	for policy, want := range table {
		if got := string(policy); got != want {
			t.Errorf("StackPolicy = %q, want %q", got, want)
		}
	}
}

func TestStackRule(t *testing.T) {
	table := []struct {
		typ  errs.ErrorType
		want errs.StackPolicy
	}{
		{errs.TypeUnknown, errs.StackMust},
		{errs.TypeInternal, errs.StackMust},
		{errs.TypeUnavailable, errs.StackMust},
		{errs.TypeDeadlineExceeded, errs.StackMust},
		{errs.TypeDataLoss, errs.StackMust},
		{errs.TypeCancelled, errs.StackOptional},
		{errs.TypeAborted, errs.StackOptional},
		{errs.TypeInvalidArgument, errs.StackNone},
		{errs.TypeFailedPrecondition, errs.StackNone},
		{errs.TypeAlreadyExists, errs.StackNone},
		{errs.TypeNotFound, errs.StackNone},
		{errs.TypePermissionDenied, errs.StackNone},
		{errs.TypeUnauthenticated, errs.StackNone},
		{errs.TypeOutOfRange, errs.StackNone},
		{errs.TypeResourceExhausted, errs.StackNone},
		{errs.TypeUnimplemented, errs.StackNone},
		{errs.ErrorType(""), errs.StackNone},
		{errs.ErrorType("NOT_A_CODE"), errs.StackNone},
	}
	for _, tc := range table {
		if got := errs.StackRule(tc.typ); got != tc.want {
			t.Errorf("StackRule(%q) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestNewValidation(t *testing.T) {
	e := errs.NewValidation("name is required")
	if e.Kind() != errs.KindValidation {
		t.Errorf("Kind() = %q, want %q", e.Kind(), errs.KindValidation)
	}
	if e.ErrorType() != errs.TypeInvalidArgument {
		t.Errorf("ErrorType() = %q, want %q", e.ErrorType(), errs.TypeInvalidArgument)
	}
	if e.ErrCode() != "" {
		t.Errorf("ErrCode() = %q, want empty", e.ErrCode())
	}
	if e.Error() != "name is required" {
		t.Errorf("Error() = %q, want %q", e.Error(), "name is required")
	}
	if e.Source() != (errs.Source{}) {
		t.Errorf("Source() = %+v, want zero value", e.Source())
	}
	var ae errs.AppError = e
	if ae.Kind() != errs.KindValidation {
		t.Errorf("AppError.Kind() = %q, want %q", ae.Kind(), errs.KindValidation)
	}
}

func TestNewBusiness(t *testing.T) {
	const code errs.ErrorCode = "ORDER.CREATE.STOCK_INSUFFICIENT"
	typ := errs.ErrorType("FAILED_PRECONDITION")
	e := errs.NewBusiness(code, typ, "库存不足")
	if e.Kind() != errs.KindBusiness {
		t.Errorf("Kind() = %q, want %q", e.Kind(), errs.KindBusiness)
	}
	if e.ErrCode() != code {
		t.Errorf("ErrCode() = %q, want %q", e.ErrCode(), code)
	}
	if e.ErrorType() != typ {
		t.Errorf("ErrorType() = %q, want %q", e.ErrorType(), typ)
	}
	if e.Error() != "库存不足" {
		t.Errorf("Error() = %q, want %q", e.Error(), "库存不足")
	}
}

func TestBizErrorWithSource(t *testing.T) {
	e := errs.NewValidation("name is required")
	s := errs.Source{Function: "order.Create", Filepath: "internal/order/create.go", Line: 42}
	withSource := e.WithSource(s)
	if withSource.Source() != s {
		t.Errorf("WithSource().Source() = %+v, want %+v", withSource.Source(), s)
	}
	// 不可变风格：原值不受影响。
	if e.Source() != (errs.Source{}) {
		t.Errorf("original Source() = %+v, want zero value", e.Source())
	}
}

func TestNewValidationError(t *testing.T) {
	cause := errors.New("invalid input")
	source := errs.Source{Function: "validate", Filepath: "input.go", Line: 9}
	e, err := errs.NewValidationError(errs.ValidationErrorConfig{Message: "  required  ", Cause: cause, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if e.Error() != "required: invalid input" || e.Kind() != errs.KindValidation || e.ErrorType() != errs.TypeInvalidArgument || e.ErrCode() != "" {
		t.Errorf("unexpected validation error: %#v, Error()=%q", e, e.Error())
	}
	if !errors.Is(e, cause) {
		t.Error("cause is not in unwrap chain")
	}
	if e.Source() != source {
		t.Errorf("Source() = %+v, want %+v", e.Source(), source)
	}
	for _, message := range []string{"", "  \t\n"} {
		if _, err := errs.NewValidationError(errs.ValidationErrorConfig{Message: message}); err == nil {
			t.Errorf("NewValidationError(message=%q) = nil error", message)
		}
	}
}

func TestNewBusinessError(t *testing.T) {
	cause := errors.New("stock is zero")
	source := errs.Source{Function: "reserve", Filepath: "stock.go", Line: 12}
	e, err := errs.NewBusinessError(errs.BusinessErrorConfig{
		Code: "ORDER.CREATE.STOCK_INSUFFICIENT", Type: "FAILED_PRECONDITION",
		Message: "  insufficient stock ", Cause: cause, Source: source,
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.Error() != "insufficient stock: stock is zero" || e.Kind() != errs.KindBusiness || e.ErrCode() != "ORDER.CREATE.STOCK_INSUFFICIENT" || e.ErrorType() != "FAILED_PRECONDITION" {
		t.Errorf("unexpected business error: %#v, Error()=%q", e, e.Error())
	}
	if !errors.Is(e, cause) || e.Source() != source {
		t.Error("cause or source was not mapped")
	}

	tests := []errs.BusinessErrorConfig{
		{Code: "", Type: "FAILED_PRECONDITION", Message: "failed"},
		{Code: "ORDER.BAD", Type: "FAILED_PRECONDITION", Message: "failed"},
		{Code: "ORDER.CREATE.FAIL", Type: "", Message: "failed"},
		{Code: "ORDER.CREATE.FAIL", Type: "db.failed", Message: "failed"},
		{Code: "ORDER.CREATE.FAIL", Type: "FAILED_PRECONDITION", Message: " "},
	}
	for _, cfg := range tests {
		if _, err := errs.NewBusinessError(cfg); err == nil {
			t.Errorf("NewBusinessError(%+v) = nil error", cfg)
		}
	}
}

func TestNewSystemDefaults(t *testing.T) {
	e := errs.NewSystem(errs.TypeUnavailable, "connect failed")
	if e.Kind() != errs.KindSystem {
		t.Errorf("Kind() = %q, want %q", e.Kind(), errs.KindSystem)
	}
	if e.ErrorType() != errs.TypeUnavailable {
		t.Errorf("ErrorType() = %q, want %q", e.ErrorType(), errs.TypeUnavailable)
	}
	if e.Error() != "connect failed" {
		t.Errorf("Error() = %q, want %q", e.Error(), "connect failed")
	}
	if e.Retryable() {
		t.Error("Retryable() = true, want false")
	}
	if e.Retries() != 0 {
		t.Errorf("Retries() = %d, want 0", e.Retries())
	}
	if e.RetriesExhausted() {
		t.Error("RetriesExhausted() = true, want false")
	}
	if e.Upstream() != "" {
		t.Errorf("Upstream() = %q, want empty", e.Upstream())
	}
	if e.ErrCode() != "" {
		t.Errorf("ErrCode() = %q, want empty", e.ErrCode())
	}
	if e.Stack() == "" {
		t.Error("Stack() empty, want 构造点自动采集（db.* 为 StackMust）")
	}
	if e.Source() != (errs.Source{}) {
		t.Errorf("Source() = %+v, want zero value", e.Source())
	}
}

func TestNewSystemOptions(t *testing.T) {
	src := errs.Source{Function: "repo.Find", Filepath: "internal/repo/find.go", Line: 7}
	stack := "goroutine 1 [running]:\ngithub.com/formal-you/go-observability/errs_test.TestNewSystemOptions"
	e := errs.NewSystem(
		errs.TypeDeadlineExceeded,
		"query timeout after 5s",
		errs.WithRetry(3, true),
		errs.WithUpstream("order-db"),
		errs.WithCode("ORDER.QUERY.TIMEOUT"),
		errs.WithStack(stack),
		errs.WithSource(src),
	)
	if e.Kind() != errs.KindSystem {
		t.Errorf("Kind() = %q, want %q", e.Kind(), errs.KindSystem)
	}
	if e.ErrorType() != errs.TypeDeadlineExceeded {
		t.Errorf("ErrorType() = %q, want %q", e.ErrorType(), errs.TypeDeadlineExceeded)
	}
	if !e.Retryable() {
		t.Error("Retryable() = false, want true")
	}
	if e.Retries() != 3 {
		t.Errorf("Retries() = %d, want 3", e.Retries())
	}
	if !e.RetriesExhausted() {
		t.Error("RetriesExhausted() = false, want true")
	}
	if e.Upstream() != "order-db" {
		t.Errorf("Upstream() = %q, want %q", e.Upstream(), "order-db")
	}
	if e.ErrCode() != "ORDER.QUERY.TIMEOUT" {
		t.Errorf("ErrCode() = %q, want %q", e.ErrCode(), "ORDER.QUERY.TIMEOUT")
	}
	if e.Stack() != stack {
		t.Errorf("Stack() = %q, want %q", e.Stack(), stack)
	}
	if e.Source() != src {
		t.Errorf("Source() = %+v, want %+v", e.Source(), src)
	}
	if e.Error() != "query timeout after 5s" {
		t.Errorf("Error() = %q, want %q", e.Error(), "query timeout after 5s")
	}
	var ae errs.AppError = e
	if ae.ErrCode() != "ORDER.QUERY.TIMEOUT" {
		t.Errorf("AppError.ErrCode() = %q, want %q", ae.ErrCode(), "ORDER.QUERY.TIMEOUT")
	}
}

func TestNewSystemError(t *testing.T) {
	cause := errors.New("deadline")
	source := errs.Source{Function: "query", Filepath: "repo.go", Line: 30}
	e, err := errs.NewSystemError(errs.SystemErrorConfig{
		Type: errs.TypeDeadlineExceeded, Code: "ORDER.QUERY.TIMEOUT", Message: "  query failed ", Cause: cause,
		Retryable: true, Retries: 3, RetriesExhausted: true, Upstream: "order-db", Stack: "custom", Source: source,
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.Error() != "query failed: deadline" || e.Kind() != errs.KindSystem || e.ErrorType() != errs.TypeDeadlineExceeded || e.ErrCode() != "ORDER.QUERY.TIMEOUT" {
		t.Errorf("unexpected system error: %#v, Error()=%q", e, e.Error())
	}
	if !e.Retryable() || e.Retries() != 3 || !e.RetriesExhausted() || e.Upstream() != "order-db" || e.Stack() != "custom" || e.Source() != source || !errors.Is(e, cause) {
		t.Error("system error config fields were not mapped")
	}

	auto, err := errs.NewSystemError(errs.SystemErrorConfig{Type: errs.TypeUnavailable, Message: "connect failed"})
	if err != nil {
		t.Fatal(err)
	}
	if auto.Stack() == "" {
		t.Error("StackMust type did not auto-capture stack")
	}
	withoutCode, err := errs.NewSystemError(errs.SystemErrorConfig{Type: errs.TypeAborted, Message: "aborted"})
	if err != nil || withoutCode.ErrCode() != "" || withoutCode.Stack() != "" {
		t.Errorf("optional code/stack defaults = code %q stack %q err %v", withoutCode.ErrCode(), withoutCode.Stack(), err)
	}

	tests := []errs.SystemErrorConfig{
		{Type: "", Message: "failed"},
		{Type: "FAILED_PRECONDITION", Message: "failed"},
		{Type: "INVALID_ARGUMENT", Message: "failed"},
		{Type: "DB.failed", Message: "failed"},
		{Type: "db.failed", Code: "ORDER.BAD", Message: "failed"},
		{Type: "db.failed", Message: " "},
		{Type: "db.failed", Message: "failed", Retries: -1},
		{Type: "db.failed", Message: "failed", Retries: 1},
		{Type: "db.failed", Message: "failed", RetriesExhausted: true},
	}
	for _, cfg := range tests {
		if _, err := errs.NewSystemError(cfg); err == nil {
			t.Errorf("NewSystemError(%+v) = nil error", cfg)
		}
	}
}

func TestLegacyConstructorsRemainPermissive(t *testing.T) {
	if got := errs.NewValidation(" ").Error(); got != " " {
		t.Errorf("NewValidation changed legacy message behavior: %q", got)
	}
	if got := errs.NewBusiness("CODE", "not.strict", ""); got.ErrCode() != "CODE" || got.Error() != "" {
		t.Errorf("NewBusiness changed legacy behavior: %#v", got)
	}
	if got := errs.NewSystem("business.too.many", "", errs.WithRetry(-1, true)); got.Retries() != -1 || !got.RetriesExhausted() {
		t.Errorf("NewSystem changed legacy behavior: %#v", got)
	}
}

func TestSystemErrorWithRetryNotExhausted(t *testing.T) {
	e := errs.NewSystem(errs.TypeDeadlineExceeded, "timeout", errs.WithRetry(1, false))
	if !e.Retryable() {
		t.Error("Retryable() = false, want true")
	}
	if e.Retries() != 1 {
		t.Errorf("Retries() = %d, want 1", e.Retries())
	}
	if e.RetriesExhausted() {
		t.Error("RetriesExhausted() = true, want false")
	}
}

func TestUnwrap(t *testing.T) {
	if got := errors.Unwrap(errs.NewValidation("v")); got != nil {
		t.Errorf("Unwrap(NewValidation) = %v, want nil", got)
	}
	if got := errors.Unwrap(errs.NewBusiness("CODE", errs.ErrorType("business.x"), "b")); got != nil {
		t.Errorf("Unwrap(NewBusiness) = %v, want nil", got)
	}
	if got := errors.Unwrap(errs.NewSystem(errs.TypeUnavailable, "s")); got != nil {
		t.Errorf("Unwrap(NewSystem) = %v, want nil", got)
	}
}

func TestIsRetryExhausted(t *testing.T) {
	exhausted := errs.NewSystem(errs.TypeResourceExhausted, "retry exhausted", errs.WithRetry(5, true))
	if !errs.IsRetryExhausted(exhausted) {
		t.Error("IsRetryExhausted(exhausted) = false, want true")
	}
	retrying := errs.NewSystem(errs.TypeDeadlineExceeded, "timeout", errs.WithRetry(1, false))
	if errs.IsRetryExhausted(retrying) {
		t.Error("IsRetryExhausted(retrying) = true, want false")
	}
	noRetry := errs.NewSystem(errs.TypeUnavailable, "connect failed")
	if errs.IsRetryExhausted(noRetry) {
		t.Error("IsRetryExhausted(noRetry) = true, want false")
	}
	if errs.IsRetryExhausted(errs.NewBusiness("CODE", errs.ErrorType("business.x"), "b")) {
		t.Error("IsRetryExhausted(BizError) = true, want false")
	}
	if errs.IsRetryExhausted(errors.New("plain error")) {
		t.Error("IsRetryExhausted(plain error) = true, want false")
	}
	// 沿错误链可识别被 %w 包裹的 SystemError。
	wrapped := fmt.Errorf("wrap: %w", exhausted)
	if !errs.IsRetryExhausted(wrapped) {
		t.Error("IsRetryExhausted(wrapped) = false, want true")
	}
	if !errs.IsRetryExhausted(&exhausted) {
		t.Error("IsRetryExhausted(&exhausted) = false, want true")
	}
	wrappedPointer := fmt.Errorf("wrap pointer: %w", &exhausted)
	if !errs.IsRetryExhausted(wrappedPointer) {
		t.Error("IsRetryExhausted(wrapped pointer) = false, want true")
	}
}

func TestCaptureSource(t *testing.T) {
	src := errs.CaptureSource(1)
	if src.Function == "" {
		t.Fatal("CaptureSource(1) returned empty Function")
	}
	if !strings.Contains(src.Function, "TestCaptureSource") {
		t.Errorf("Function = %q, want to contain %q", src.Function, "TestCaptureSource")
	}
	if src.Filepath == "" {
		t.Error("Filepath is empty, want non-empty")
	}
	if src.Line <= 0 {
		t.Errorf("Line = %d, want > 0", src.Line)
	}
	// skip=0 定位 CaptureSource 自身。
	self := errs.CaptureSource(0)
	if !strings.Contains(self.Function, "errs.CaptureSource") {
		t.Errorf("CaptureSource(0) Function = %q, want to contain %q", self.Function, "errs.CaptureSource")
	}
	// skip=2 定位调用方上层，函数名非空即可。
	outer := errs.CaptureSource(2)
	if outer.Function == "" {
		t.Error("CaptureSource(2) returned empty Function")
	}
}

func TestCaptureStack(t *testing.T) {
	stack := errs.CaptureStack()
	if stack == "" {
		t.Fatal("CaptureStack() returned empty string")
	}
	if !strings.Contains(stack, "errs.CaptureStack") {
		t.Errorf("stack does not contain errs.CaptureStack:\n%s", stack)
	}
	if !strings.Contains(stack, "TestCaptureStack") {
		t.Errorf("stack does not contain TestCaptureStack:\n%s", stack)
	}
}

func TestNewSystemStackPolicy(t *testing.T) {
	t.Run("must auto-captures at creation", func(t *testing.T) {
		e := errs.NewSystem(errs.TypeDeadlineExceeded, "timeout")
		if e.Stack() == "" {
			t.Fatal("Stack() empty, want StackMust 类别构造点自动采集")
		}
		if !strings.Contains(e.Stack(), "TestNewSystemStackPolicy") {
			t.Errorf("堆栈应包含创建点调用帧 TestNewSystemStackPolicy:\n%s", e.Stack())
		}
	})
	t.Run("optional does not auto-capture", func(t *testing.T) {
		e := errs.NewSystem(errs.TypeCancelled, "canceled")
		if e.Stack() != "" {
			t.Errorf("Stack() = %q, want empty for StackOptional", e.Stack())
		}
	})
	t.Run("none does not auto-capture", func(t *testing.T) {
		e := errs.NewSystem(errs.TypeFailedPrecondition, "denied")
		if e.Stack() != "" {
			t.Errorf("Stack() = %q, want empty for StackNone", e.Stack())
		}
	})
	t.Run("explicit WithStack wins over auto-capture", func(t *testing.T) {
		e := errs.NewSystem(errs.TypeUnavailable, "conn", errs.WithStack("custom"))
		if e.Stack() != "custom" {
			t.Errorf("Stack() = %q, want explicit custom", e.Stack())
		}
	})
}

func TestSetStackPolicy(t *testing.T) {
	t.Cleanup(func() { errs.SetStackPolicy(nil) })

	t.Run("default without overrides", func(t *testing.T) {
		if got := errs.StackRule(errs.TypeDeadlineExceeded); got != errs.StackMust {
			t.Fatalf("StackRule(DEADLINE_EXCEEDED) = %q, want must", got)
		}
	})

	t.Run("exact override to none disables auto capture", func(t *testing.T) {
		errs.SetStackPolicy(map[string]errs.StackPolicy{string(errs.TypeDeadlineExceeded): errs.StackNone})
		if got := errs.StackRule(errs.TypeDeadlineExceeded); got != errs.StackNone {
			t.Fatalf("StackRule(DEADLINE_EXCEEDED) = %q, want none", got)
		}
		if got := errs.StackRule(errs.TypeUnavailable); got != errs.StackMust {
			t.Fatalf("StackRule(UNAVAILABLE) = %q, want must（未覆盖仍走内置默认）", got)
		}
		e := errs.NewSystem(errs.TypeDeadlineExceeded, "timeout")
		if e.Stack() != "" {
			t.Fatalf("Stack() = %q, want empty（DEADLINE_EXCEEDED 覆盖为 none 不自动采集）", e.Stack())
		}
	})

	t.Run("override exact code to must enables auto capture", func(t *testing.T) {
		errs.SetStackPolicy(map[string]errs.StackPolicy{string(errs.TypeCancelled): errs.StackMust})
		if got := errs.StackRule(errs.TypeCancelled); got != errs.StackMust {
			t.Fatalf("StackRule(CANCELLED) = %q, want must", got)
		}
		e := errs.NewSystem(errs.TypeCancelled, "canceled")
		if e.Stack() == "" {
			t.Fatal("Stack() empty, want 自动采集（覆盖为 must）")
		}
	})

	t.Run("exact match wins over default", func(t *testing.T) {
		errs.SetStackPolicy(map[string]errs.StackPolicy{
			string(errs.TypeDeadlineExceeded): errs.StackMust,
			string(errs.TypeUnavailable):      errs.StackNone,
		})
		if got := errs.StackRule(errs.TypeDeadlineExceeded); got != errs.StackMust {
			t.Fatalf("StackRule(DEADLINE_EXCEEDED) = %q, want must", got)
		}
		if got := errs.StackRule(errs.TypeUnavailable); got != errs.StackNone {
			t.Fatalf("StackRule(UNAVAILABLE) = %q, want none", got)
		}
	})

	t.Run("reset restores default", func(t *testing.T) {
		errs.SetStackPolicy(map[string]errs.StackPolicy{string(errs.TypeDeadlineExceeded): errs.StackNone})
		errs.SetStackPolicy(nil)
		if got := errs.StackRule(errs.TypeDeadlineExceeded); got != errs.StackMust {
			t.Fatalf("StackRule(DEADLINE_EXCEEDED) = %q, want must（重置后回默认）", got)
		}
	})
}
