package errs_test

import (
	"errors"
	"testing"

	"github.com/formal-you/go-observability/errs"
)

func TestErrorCodeStrictContract(t *testing.T) {
	valid := []string{
		"ORDER.CREATE.STOCK_INSUFFICIENT",
		"ORDER_API.PAYMENT_V2.UPSTREAM_5XX",
		"1ORDER.CREATE.FAILED",
	}
	for _, raw := range valid {
		code, err := errs.ParseErrorCode(raw)
		if err != nil || code != errs.ErrorCode(raw) {
			t.Errorf("ParseErrorCode(%q) = %q, %v", raw, code, err)
		}
	}

	invalid := []string{
		"", "ORDER.CREATE", "ORDER.CREATE.STOCK.INSUFFICIENT",
		"order.CREATE.FAILED", "ORDER.CREATE.stock",
		"ORDER.CREATE.BAD-REASON", " ORDER.CREATE.FAILED ",
	}
	for _, raw := range invalid {
		if got, err := errs.ParseErrorCode(raw); err == nil || got != "" {
			t.Errorf("ParseErrorCode(%q) = %q, %v; want zero and error", raw, got, err)
		}
	}
}

func TestErrorTypeStrictContract(t *testing.T) {
	valid := []string{"FAILED_PRECONDITION", "DEADLINE_EXCEEDED", "UNAVAILABLE"}
	for _, raw := range valid {
		typ, err := errs.ParseErrorType(raw)
		if err != nil || typ != errs.ErrorType(raw) {
			t.Errorf("ParseErrorType(%q) = %q, %v", raw, typ, err)
		}
	}

	invalid := []string{
		"", "business", "unavailable", "business.order.stock_insufficient",
		"Business.failed", "business.STOCK", "business.bad-reason", " business.failed ",
	}
	for _, raw := range invalid {
		if got, err := errs.ParseErrorType(raw); err == nil || got != "" {
			t.Errorf("ParseErrorType(%q) = %q, %v; want zero and error", raw, got, err)
		}
	}
}

func TestStrictConstructorsPreserveCause(t *testing.T) {
	cause := errors.New("root cause")
	business, err := errs.NewBusinessError(errs.BusinessErrorConfig{
		Code: "ORDER.CREATE.STOCK_INSUFFICIENT", Type: "FAILED_PRECONDITION",
		Message: " stock insufficient ", Cause: cause,
	})
	if err != nil || business.Error() != "stock insufficient: root cause" || !errors.Is(business, cause) {
		t.Fatalf("business = %#v, err=%v", business, err)
	}

	system, err := errs.NewSystemError(errs.SystemErrorConfig{
		Type: errs.TypeDeadlineExceeded, Message: " query failed ", Cause: cause,
		Retryable: true, Retries: 2, RetriesExhausted: true,
	})
	if err != nil || !system.Retryable() || system.Retries() != 2 || !system.RetriesExhausted() || !errors.Is(system, cause) {
		t.Fatalf("system = %#v, err=%v", system, err)
	}
}

func TestStrictConstructorsRejectInvalidSemantics(t *testing.T) {
	tests := []struct {
		name string
		call func() error
	}{
		{"empty validation message", func() error { _, err := errs.NewValidationError(errs.ValidationErrorConfig{}); return err }},
		{"business code required", func() error {
			_, err := errs.NewBusinessError(errs.BusinessErrorConfig{Type: "FAILED_PRECONDITION", Message: "x"})
			return err
		}},
		{"business namespace required", func() error {
			_, err := errs.NewBusinessError(errs.BusinessErrorConfig{Code: "ORDER.CREATE.FAILED", Type: errs.TypeDeadlineExceeded, Message: "x"})
			return err
		}},
		{"system business namespace rejected", func() error {
			_, err := errs.NewSystemError(errs.SystemErrorConfig{Type: "FAILED_PRECONDITION", Message: "x"})
			return err
		}},
		{"negative retries", func() error {
			_, err := errs.NewSystemError(errs.SystemErrorConfig{Type: errs.TypeDeadlineExceeded, Message: "x", Retryable: true, Retries: -1})
			return err
		}},
		{"non retryable metadata", func() error {
			_, err := errs.NewSystemError(errs.SystemErrorConfig{Type: errs.TypeDeadlineExceeded, Message: "x", Retries: 1})
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("strict constructor error = nil")
			}
		})
	}
}

func TestLegacyConstructorsRemainCompatible(t *testing.T) {
	business := errs.NewBusiness("legacy-code", "business.too.many", "")
	system := errs.NewSystem("business.too.many", "", errs.WithRetry(-1, true))
	if business.ErrCode() != "legacy-code" || system.Retries() != -1 || !system.RetriesExhausted() {
		t.Fatal("legacy constructors changed permissive behavior")
	}
}
