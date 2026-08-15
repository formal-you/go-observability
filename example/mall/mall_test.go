package mall_test

import (
	"log/slog"
	"testing"

	"github.com/formal-you/go-observability/example/mall"
	log "github.com/formal-you/go-observability/log"
)

// TestB4Registry 验收 CASE-B4-02/03：10 个业务名合法且不含 order.rejected。
func TestB4Registry(t *testing.T) {
	names := mall.AllBusinessEvents()
	if len(names) != 10 {
		t.Fatalf("business 事件应为 10 个，实际 %d（Oracle: RULE-B4-01）", len(names))
	}
	for _, n := range names {
		if err := n.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil（Oracle: ACCEPT-B4-01）", n, err)
		}
		if n == "business.order.rejected" {
			t.Error("order.rejected 不应出现（RULE-B4-01）")
		}
	}
}

// TestEventNameForErrorCode 验收业务错误码到领域错误事件名的接入方注册表映射。
func TestEventNameForErrorCode(t *testing.T) {
	name, ok := mall.EventNameForErrorCode("ORDER.CREATE.STOCK_INSUFFICIENT")
	if !ok || name != mall.EventOrderCreateStockInsufficient {
		t.Fatalf("EventNameForErrorCode(ORDER.CREATE.STOCK_INSUFFICIENT) = %q, %v; want %q, true",
			name, ok, mall.EventOrderCreateStockInsufficient)
	}
	if _, ok := mall.EventNameForErrorCode("ORDER.UNKNOWN.NOT_REGISTERED"); ok {
		t.Error("unknown code should return false")
	}
}

// TestOrderPaidExtraAttrs 验收 CASE-B4-01：ExtraAttrs 扁平含 app.order_id 等。
func TestOrderPaidExtraAttrs(t *testing.T) {
	ev := log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName:  mall.EventOrderPaid,
			Result:     log.ResultSuccess,
			ExtraAttrs: mall.OrderPaidExtra("ORD-1", "wechat", "2026-08-09T10:00:00+08:00", 9900),
		},
	}
	m := map[string]slog.Value{}
	for _, a := range ev.Attrs() {
		m[a.Key] = a.Value
	}
	for key, want := range map[string]string{
		"app.order_id":    "ORD-1",
		"app.pay_channel": "wechat",
		"app.paid_at":     "2026-08-09T10:00:00+08:00",
	} {
		if got := m[key].String(); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if got := m["app.amount"].Int64(); got != 9900 {
		t.Errorf("app.amount = %d, want 9900", got)
	}
}
