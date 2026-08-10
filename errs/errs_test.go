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
		errs.TypeValidationFailed:        "validation.failed",
		errs.TypeDBConnectionError:       "db.connection_error",
		errs.TypeDBQueryTimeout:          "db.query_timeout",
		errs.TypeDBDeadlock:              "db.deadlock",
		errs.TypeRedisConnectionError:    "redis.connection_error",
		errs.TypeRedisTimeout:            "redis.timeout",
		errs.TypeMQPublishFailed:         "mq.publish_failed",
		errs.TypeMQConsumeFailed:         "mq.consume_failed",
		errs.TypeMQRetryExhausted:        "mq.retry_exhausted",
		errs.TypeHTTPUpstream5xx:         "http.upstream_5xx",
		errs.TypeHTTPUpstreamTimeout:     "http.upstream_timeout",
		errs.TypeLockConflict:            "lock.conflict",
		errs.TypeIdempotencyConflict:     "idempotency.conflict",
		errs.TypeStockRace:               "stock.race",
		errs.TypeDataJSONUnmarshal:       "data.json_unmarshal",
		errs.TypeDataDuplicateKey:        "data.duplicate_key",
		errs.TypeDataNotFound:            "data.not_found",
		errs.TypeRuntimePanic:            "runtime.panic",
		errs.TypeRuntimeContextCanceled:  "runtime.context_cancelled",
		errs.TypeRuntimeDeadlineExceeded: "runtime.deadline_exceeded",
	}
	for typ, want := range table {
		if got := string(typ); got != want {
			t.Errorf("ErrorType = %q, want %q", got, want)
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
		{errs.TypeRuntimePanic, errs.StackMust},
		{errs.TypeRuntimeContextCanceled, errs.StackOptional},
		{errs.TypeRuntimeDeadlineExceeded, errs.StackMust},
		{errs.TypeDBConnectionError, errs.StackMust},
		{errs.TypeDBQueryTimeout, errs.StackMust},
		{errs.TypeDBDeadlock, errs.StackMust},
		{errs.TypeRedisConnectionError, errs.StackMust},
		{errs.TypeRedisTimeout, errs.StackMust},
		{errs.TypeMQPublishFailed, errs.StackMust},
		{errs.TypeMQConsumeFailed, errs.StackMust},
		{errs.TypeMQRetryExhausted, errs.StackMust},
		{errs.TypeHTTPUpstream5xx, errs.StackMust},
		{errs.TypeHTTPUpstreamTimeout, errs.StackMust},
		{errs.TypeLockConflict, errs.StackOptional},
		{errs.TypeIdempotencyConflict, errs.StackOptional},
		{errs.TypeStockRace, errs.StackOptional},
		{errs.TypeDataJSONUnmarshal, errs.StackOptional},
		{errs.TypeDataDuplicateKey, errs.StackOptional},
		{errs.TypeDataNotFound, errs.StackOptional},
		{errs.TypeValidationFailed, errs.StackNone},
		{errs.ErrorType("business.stock_insufficient"), errs.StackNone},
		{errs.ErrorType("validation.required"), errs.StackNone},
		{errs.ErrorType(""), errs.StackNone},
		{errs.ErrorType("unknown.category"), errs.StackNone},
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
	if e.ErrorType() != errs.TypeValidationFailed {
		t.Errorf("ErrorType() = %q, want %q", e.ErrorType(), errs.TypeValidationFailed)
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
	typ := errs.ErrorType("business.stock_insufficient")
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

func TestNewSystemDefaults(t *testing.T) {
	e := errs.NewSystem(errs.TypeDBConnectionError, "connect failed")
	if e.Kind() != errs.KindSystem {
		t.Errorf("Kind() = %q, want %q", e.Kind(), errs.KindSystem)
	}
	if e.ErrorType() != errs.TypeDBConnectionError {
		t.Errorf("ErrorType() = %q, want %q", e.ErrorType(), errs.TypeDBConnectionError)
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
		errs.TypeDBQueryTimeout,
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
	if e.ErrorType() != errs.TypeDBQueryTimeout {
		t.Errorf("ErrorType() = %q, want %q", e.ErrorType(), errs.TypeDBQueryTimeout)
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

func TestSystemErrorWithRetryNotExhausted(t *testing.T) {
	e := errs.NewSystem(errs.TypeRedisTimeout, "timeout", errs.WithRetry(1, false))
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
	if got := errors.Unwrap(errs.NewSystem(errs.TypeDBConnectionError, "s")); got != nil {
		t.Errorf("Unwrap(NewSystem) = %v, want nil", got)
	}
}

func TestIsRetryExhausted(t *testing.T) {
	exhausted := errs.NewSystem(errs.TypeMQRetryExhausted, "retry exhausted", errs.WithRetry(5, true))
	if !errs.IsRetryExhausted(exhausted) {
		t.Error("IsRetryExhausted(exhausted) = false, want true")
	}
	retrying := errs.NewSystem(errs.TypeDBQueryTimeout, "timeout", errs.WithRetry(1, false))
	if errs.IsRetryExhausted(retrying) {
		t.Error("IsRetryExhausted(retrying) = true, want false")
	}
	noRetry := errs.NewSystem(errs.TypeDBConnectionError, "connect failed")
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
		e := errs.NewSystem(errs.TypeDBQueryTimeout, "timeout")
		if e.Stack() == "" {
			t.Fatal("Stack() empty, want StackMust 类别构造点自动采集")
		}
		if !strings.Contains(e.Stack(), "TestNewSystemStackPolicy") {
			t.Errorf("堆栈应包含创建点调用帧 TestNewSystemStackPolicy:\n%s", e.Stack())
		}
	})
	t.Run("optional does not auto-capture", func(t *testing.T) {
		e := errs.NewSystem(errs.TypeRuntimeContextCanceled, "canceled")
		if e.Stack() != "" {
			t.Errorf("Stack() = %q, want empty for StackOptional", e.Stack())
		}
	})
	t.Run("none does not auto-capture", func(t *testing.T) {
		e := errs.NewSystem(errs.ErrorType("business.auth.forbidden"), "denied")
		if e.Stack() != "" {
			t.Errorf("Stack() = %q, want empty for StackNone", e.Stack())
		}
	})
	t.Run("explicit WithStack wins over auto-capture", func(t *testing.T) {
		e := errs.NewSystem(errs.TypeDBConnectionError, "conn", errs.WithStack("custom"))
		if e.Stack() != "custom" {
			t.Errorf("Stack() = %q, want explicit custom", e.Stack())
		}
	})
}

func TestSetStackPolicy(t *testing.T) {
	t.Cleanup(func() { errs.SetStackPolicy(nil) })

	t.Run("default without overrides", func(t *testing.T) {
		if got := errs.StackRule(errs.TypeDBQueryTimeout); got != errs.StackMust {
			t.Fatalf("StackRule(db.query_timeout) = %q, want must", got)
		}
	})

	t.Run("override db. to none disables auto capture", func(t *testing.T) {
		errs.SetStackPolicy(map[string]errs.StackPolicy{"db.": errs.StackNone})
		if got := errs.StackRule(errs.TypeDBQueryTimeout); got != errs.StackNone {
			t.Fatalf("StackRule(db.query_timeout) = %q, want none", got)
		}
		if got := errs.StackRule(errs.TypeDBConnectionError); got != errs.StackNone {
			t.Fatalf("StackRule(db.connection_error) = %q, want none", got)
		}
		e := errs.NewSystem(errs.TypeDBQueryTimeout, "timeout")
		if e.Stack() != "" {
			t.Fatalf("Stack() = %q, want empty（db. 覆盖为 none 不自动采集）", e.Stack())
		}
		if got := errs.StackRule(errs.TypeRedisTimeout); got != errs.StackMust {
			t.Fatalf("StackRule(redis.timeout) = %q, want must（未覆盖仍走内置默认）", got)
		}
	})

	t.Run("override exact type to must enables auto capture", func(t *testing.T) {
		errs.SetStackPolicy(map[string]errs.StackPolicy{"runtime.context_cancelled": errs.StackMust})
		if got := errs.StackRule(errs.TypeRuntimeContextCanceled); got != errs.StackMust {
			t.Fatalf("StackRule(runtime.context_cancelled) = %q, want must", got)
		}
		e := errs.NewSystem(errs.TypeRuntimeContextCanceled, "canceled")
		if e.Stack() == "" {
			t.Fatal("Stack() empty, want 自动采集（覆盖为 must）")
		}
	})

	t.Run("longest prefix wins", func(t *testing.T) {
		errs.SetStackPolicy(map[string]errs.StackPolicy{
			"db.":              errs.StackNone,
			"db.query_timeout": errs.StackMust,
		})
		if got := errs.StackRule(errs.TypeDBQueryTimeout); got != errs.StackMust {
			t.Fatalf("StackRule(db.query_timeout) = %q, want must（最长前缀优先）", got)
		}
		if got := errs.StackRule(errs.TypeDBConnectionError); got != errs.StackNone {
			t.Fatalf("StackRule(db.connection_error) = %q, want none", got)
		}
	})

	t.Run("reset restores default", func(t *testing.T) {
		errs.SetStackPolicy(map[string]errs.StackPolicy{"db.": errs.StackNone})
		errs.SetStackPolicy(nil)
		if got := errs.StackRule(errs.TypeDBQueryTimeout); got != errs.StackMust {
			t.Fatalf("StackRule(db.query_timeout) = %q, want must（重置后回默认）", got)
		}
	})
}
