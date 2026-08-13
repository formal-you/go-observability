package errs_test

import (
	"testing"

	"github.com/formal-you/go-observability/errs"
)

// TestErrorRegistryRegisterAndLookup 验收 Error Registry 的基本注册与查询：
// RegisterErrorCode 建立 ErrorCode → ErrorType 固定映射，RegisteredErrorType 可反查。
func TestErrorRegistryRegisterAndLookup(t *testing.T) {
	const (
		code errs.ErrorCode = "PAYMENT.REFUND.INSUFFICIENT_BALANCE"
		typ                 = errs.ErrorType("FAILED_PRECONDITION")
	)
	if err := errs.RegisterErrorCode(code, typ); err != nil {
		t.Fatalf("RegisterErrorCode(%q, %q) = %v", code, typ, err)
	}
	got, ok := code.RegisteredErrorType()
	if !ok || got != typ {
		t.Fatalf("RegisteredErrorType() = %q, %v; want %q, true", got, ok, typ)
	}
	if _, ok := (errs.ErrorCode("ORDER.UNKNOWN.NOT_REGISTERED")).RegisteredErrorType(); ok {
		t.Error("未注册 code 的 RegisteredErrorType 应返回 false")
	}
}

// TestErrorRegistryRejectsInvalidInput 验收非法入参在注册期即被拒绝。
func TestErrorRegistryRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		code errs.ErrorCode
		typ  errs.ErrorType
	}{
		{"empty code", "", "FAILED_PRECONDITION"},
		{"two segment code", "ORDER.FAIL", "FAILED_PRECONDITION"},
		{"lowercase code", "order.create.failed", "FAILED_PRECONDITION"},
		{"empty type", "ORDER.CREATE.FAILED", ""},
		{"three segment type", "ORDER.CREATE.FAILED", "business.too.many"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := errs.RegisterErrorCode(tc.code, tc.typ); err == nil {
				t.Fatalf("RegisterErrorCode(%q, %q) = nil, want error", tc.code, tc.typ)
			}
		})
	}
}

// TestErrorRegistryConflict 验收同一 code 重复注册的幂等与冲突规则：
// 相同映射幂等成功；不同映射返回错误（error.code → exactly one error.type）。
func TestErrorRegistryConflict(t *testing.T) {
	const code errs.ErrorCode = "PAYMENT.CAPTURE.UPSTREAM_TIMEOUT"
	if err := errs.RegisterErrorCode(code, "DEADLINE_EXCEEDED"); err != nil {
		t.Fatalf("首次注册 = %v", err)
	}
	if err := errs.RegisterErrorCode(code, "DEADLINE_EXCEEDED"); err != nil {
		t.Fatalf("相同映射重复注册 = %v, want nil", err)
	}
	if err := errs.RegisterErrorCode(code, "UNAVAILABLE"); err == nil {
		t.Fatal("不同映射重复注册 = nil, want error")
	}
}

// TestStrictConstructorsEnforceRegisteredCodeType 验收严格构造器对已注册 code 强制类型一致。
func TestStrictConstructorsEnforceRegisteredCodeType(t *testing.T) {
	const (
		code errs.ErrorCode = "ORDER.PAYMENT.UPSTREAM_5XX"
		typ                 = errs.ErrorType("UNAVAILABLE")
	)
	if err := errs.RegisterErrorCode(code, typ); err != nil {
		t.Fatalf("RegisterErrorCode = %v", err)
	}

	if _, err := errs.NewBusinessError(errs.BusinessErrorConfig{
		Code: code, Type: "FAILED_PRECONDITION", Message: "x",
	}); err == nil {
		t.Fatal("NewBusinessError 已注册 code 使用不同 type = nil, want error")
	}
	if _, err := errs.NewSystemError(errs.SystemErrorConfig{
		Type: errs.TypeDeadlineExceeded, Code: code, Message: "x",
	}); err == nil {
		t.Fatal("NewSystemError 已注册 code 使用不同 type = nil, want error")
	}

	// SystemError 允许关联非 business 命名空间，只要与注册映射一致。
	sys, err := errs.NewSystemError(errs.SystemErrorConfig{Type: typ, Code: code, Message: "x"})
	if err != nil || sys.ErrCode() != code || sys.ErrorType() != typ {
		t.Fatalf("NewSystemError 匹配映射 = %#v, err=%v", sys, err)
	}
}

// TestUnregisteredCodeRemainsPermissive 验收未注册 code 不强制映射（注册是使用方显式选择）。
func TestUnregisteredCodeRemainsPermissive(t *testing.T) {
	if _, err := errs.NewBusinessError(errs.BusinessErrorConfig{
		Code: "ORDER.QUERY.NOT_REGISTERED", Type: "FAILED_PRECONDITION", Message: "x",
	}); err != nil {
		t.Fatalf("未注册 code 的严格构造 = %v, want nil", err)
	}
	if _, err := errs.NewSystemError(errs.SystemErrorConfig{
		Type: errs.TypeDeadlineExceeded, Code: "ORDER.QUERY.NOT_REGISTERED", Message: "x",
	}); err != nil {
		t.Fatalf("未注册 code 的严格构造 = %v, want nil", err)
	}
}

// TestLegacyConstructorsSkipRegistryCheck 验收旧构造器保持宽松：不做注册映射校验。
func TestLegacyConstructorsSkipRegistryCheck(t *testing.T) {
	const code errs.ErrorCode = "ORDER.REFUND.LEGACY_CODE"
	if err := errs.RegisterErrorCode(code, "NOT_FOUND"); err != nil {
		t.Fatalf("RegisterErrorCode = %v", err)
	}
	legacy := errs.NewBusiness(code, "business.other_failed", "x")
	if legacy.ErrCode() != code || legacy.ErrorType() != "business.other_failed" {
		t.Fatal("NewBusiness 不应被注册映射拦截")
	}
	sys := errs.NewSystem(errs.TypeDeadlineExceeded, "x", errs.WithCode(code))
	if sys.ErrCode() != code {
		t.Fatal("NewSystem + WithCode 不应被注册映射拦截")
	}
}
