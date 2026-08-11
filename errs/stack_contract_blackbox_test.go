package errs_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/formal-you/go-observability/errs"
)

func TestBoundedStackContractBlackBox(t *testing.T) {
	original := errs.CurrentStackConfig()
	t.Cleanup(func() {
		if err := errs.SetStackConfig(original); err != nil {
			t.Fatalf("restore stack config: %v", err)
		}
	})

	config := errs.DevelopmentStackConfig()
	config.MaxBytes = 128
	config.PathPolicy = errs.StackPathBase
	if err := errs.SetStackConfig(config); err != nil {
		t.Fatalf("SetStackConfig: %v", err)
	}

	stack := "goroutine 1 [running]:\nmain.run()\n\tC:/workspace/private/service/handler.go:42 +0x1\n" + strings.Repeat("界", 100)
	errValue, err := errs.NewSystemError(errs.SystemErrorConfig{
		Type:    errs.TypeRuntimePanic,
		Message: "panic",
		Stack:   stack,
	})
	if err != nil {
		t.Fatalf("NewSystemError: %v", err)
	}
	if len(errValue.Stack()) > config.MaxBytes {
		t.Fatalf("stack bytes = %d, max = %d", len(errValue.Stack()), config.MaxBytes)
	}
	if !utf8.ValidString(errValue.Stack()) {
		t.Fatal("truncated stack must remain valid UTF-8")
	}
	if !errValue.StackTruncated() || !strings.Contains(errValue.Stack(), errs.StackTruncationMarker) {
		t.Fatalf("truncated stack missing marker: %q", errValue.Stack())
	}
	if strings.Contains(errValue.Stack(), "C:/workspace/private") || !strings.Contains(errValue.Stack(), "handler.go:42") {
		t.Fatalf("base path policy not applied: %q", errValue.Stack())
	}
}

func TestProductionStackProfileKeepsPanicBlackBox(t *testing.T) {
	config := errs.ProductionStackConfig()
	if err := errs.SetStackConfig(config); err != nil {
		t.Fatalf("production config: %v", err)
	}
	t.Cleanup(func() { _ = errs.SetStackConfig(errs.DevelopmentStackConfig()) })

	config.Overrides["runtime."] = errs.StackNone
	if err := errs.SetStackConfig(config); err == nil {
		t.Fatal("configuration that disables runtime.panic must be rejected")
	}
}
