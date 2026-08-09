// Package log_test 是根 log 包的外部黑盒测试层（package log_test，非白盒）。
// 期望值来自 observability-design/spec/acceptance.md 的验收契约（Oracle），
// 不读取实现内部——防"同代理写实现+测试"的自证幻觉。用例引用 RULE/ACCEPT/CASE ID。
package log_test

import (
	"testing"

	"github.com/formal-you/go-observability"
	"github.com/formal-you/go-observability/errs"
)

// TestLevelOfBlackBox 按验收契约 CASE-B3-01..05 验证 LevelOf 规则表。
// 期望值来源：B3 决策表（spec/acceptance.md），非实现代码。
func TestLevelOfBlackBox(t *testing.T) {
	cases := []struct {
		name string
		err  errs.AppError
		want log.Level
	}{
		{"CASE-B3-01 validation → WARN", errs.NewValidation("v"), log.LevelWarn},
		{"CASE-B3-02 business → WARN", errs.NewBusiness("C", errs.ErrorType("business.x"), "b"), log.LevelWarn},
		{"CASE-B3-03 system retrying → WARN", errs.NewSystem(errs.TypeDBQueryTimeout, "t", errs.WithRetry(1, false)), log.LevelWarn},
		{"CASE-B3-04 system exhausted → ERROR", errs.NewSystem(errs.TypeMQRetryExhausted, "e", errs.WithRetry(5, true)), log.LevelError},
		{"CASE-B3-05 system not retryable → ERROR", errs.NewSystem(errs.TypeRuntimePanic, "p"), log.LevelError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := log.LevelOf(tc.err); got != tc.want {
				t.Errorf("LevelOf() = %q, want %q（Oracle: %s）", got, tc.want, tc.name)
			}
		})
	}
}

// TestEventNameValidateBlackBox 按验收契约 CASE-B1-01..05 验证 EventName 三段式文法。
// 输入字符串是 Validate 的黑盒测试数据，不是事件埋点（AGENTS.md 规则 1 约束生产侧）。
func TestEventNameValidateBlackBox(t *testing.T) {
	valid := []log.EventName{
		"business.order.paid", // CASE-B1-01
		"access.http.request", // CASE-B1-02
	}
	for _, name := range valid {
		if err := name.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil（Oracle: ACCEPT-B1-01）", name, err)
		}
	}

	invalid := []log.EventName{
		"order.paid",          // CASE-B1-03 段数不足
		"business..paid",      // CASE-B1-04 空段
		"Business.Order.Paid", // CASE-B1-05 大写
	}
	for _, name := range invalid {
		if err := name.Validate(); err == nil {
			t.Errorf("Validate(%q) = nil, want error（Oracle: ACCEPT-B1-02）", name)
		}
	}
}
