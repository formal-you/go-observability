// Package log_test 是根 log 包的外部黑盒测试层（package log_test，非白盒）。
// 期望值来自 observability-design/spec/acceptance.md 的验收契约（Oracle），
// 不读取实现内部——防"同代理写实现+测试"的自证幻觉。用例引用 RULE/ACCEPT/CASE ID。
package log_test

import (
	"log/slog"
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

// TestBusinessEventExtraAttrsBlackBox 按验收契约 CASE-B4-01 验证 ExtraAttrs 扁平输出：
// order.paid 注入 order_id/amount/pay_channel/paid_at → 扁平 attrs 含对应 mall.* 键。
func TestBusinessEventExtraAttrsBlackBox(t *testing.T) {
	ev := log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.EventNameBusinessOrderPaid,
			Result:    log.ResultSuccess,
			ExtraAttrs: []slog.Attr{
				slog.String(string(log.KeyMallOrderID), "ORD-1"),
				slog.Int64(string(log.KeyMallAmount), 9900),
				slog.String(string(log.KeyMallPayChannel), "wechat"),
				slog.String(string(log.KeyMallPaidAt), "2026-08-09T10:00:00+08:00"),
			},
		},
	}
	m := map[string]slog.Value{}
	for _, a := range ev.Attrs() {
		m[a.Key] = a.Value
	}
	for key, want := range map[string]string{
		"mall.order_id":    "ORD-1",
		"mall.pay_channel": "wechat",
		"mall.paid_at":     "2026-08-09T10:00:00+08:00",
	} {
		if got := m[key].String(); got != want {
			t.Errorf("%s = %q, want %q（Oracle: ACCEPT-B4-02）", key, got, want)
		}
	}
	if got := m["mall.amount"].Int64(); got != 9900 {
		t.Errorf("mall.amount = %d, want 9900（Oracle: ACCEPT-B4-03 金额整数分）", got)
	}
}

// TestBusinessEventNamesBlackBox 按验收契约 CASE-B4-02/03 验证 B4 定稿的 10 个
// business.* 事件名三段式合法且为注册表全集（order.rejected 有意不埋，RULE-B4-01）。
func TestBusinessEventNamesBlackBox(t *testing.T) {
	names := []log.EventName{
		log.EventNameBusinessOrderCreated,
		log.EventNameBusinessOrderPaid,
		log.EventNameBusinessOrderCancelled,
		log.EventNameBusinessRefundCreated,
		log.EventNameBusinessRefundSuccess,
		log.EventNameBusinessCartAdded,
		log.EventNameBusinessProductViewed,
		log.EventNameBusinessUserLogin,
		log.EventNameBusinessUserRegistered,
		log.EventNameBusinessCouponRedeemed,
	}
	if len(names) != 10 {
		t.Fatalf("business 事件应为 10 个，实际 %d（Oracle: RULE-B4-01）", len(names))
	}
	for _, n := range names {
		if err := n.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil（Oracle: ACCEPT-B4-01）", n, err)
		}
	}
}
