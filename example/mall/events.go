// Package mall 演示接入方如何自建业务事件注册表与专属键（C2 / B4）。
// 核心库只提供 BusinessPayload + ExtraAttrs + EventName 文法；领域名不进 types.go。
package mall

import (
	"log/slog"

	log "github.com/formal-you/go-observability/log"
)

// 业务事件名（B4 核心 10；不埋 order.rejected）。
const (
	EventOrderCreated   log.EventName = "business.order.created"
	EventOrderPaid      log.EventName = "business.order.paid"
	EventOrderCancelled log.EventName = "business.order.cancelled"
	EventRefundCreated  log.EventName = "business.refund.created"
	EventRefundSuccess  log.EventName = "business.refund.success"
	EventCartAdded      log.EventName = "business.cart.added"
	EventProductViewed  log.EventName = "business.product.viewed"
	EventUserLogin      log.EventName = "business.user.login"
	EventUserRegistered log.EventName = "business.user.registered"
	EventCouponRedeemed log.EventName = "business.coupon.redeemed"
)

// AllBusinessEvents 返回 B4 清单全集（测试/文档枚举用）。
func AllBusinessEvents() []log.EventName {
	return []log.EventName{
		EventOrderCreated,
		EventOrderPaid,
		EventOrderCancelled,
		EventRefundCreated,
		EventRefundSuccess,
		EventCartAdded,
		EventProductViewed,
		EventUserLogin,
		EventUserRegistered,
		EventCouponRedeemed,
	}
}

// 事件专属键（app.* vendor；金额分 int64；时间 RFC3339）。
const (
	KeyOrderID         log.Key = "app.order_id"
	KeyAmount          log.Key = "app.amount"
	KeyItemCount       log.Key = "app.item_count"
	KeyChannel         log.Key = "app.channel"
	KeyPayChannel      log.Key = "app.pay_channel"
	KeyPaidAt          log.Key = "app.paid_at"
	KeyCancelledAt     log.Key = "app.cancelled_at"
	KeyRefundID        log.Key = "app.refund_id"
	KeyRefundedAt      log.Key = "app.refunded_at"
	KeyProductID       log.Key = "app.product_id"
	KeyQuantity        log.Key = "app.quantity"
	KeyCategoryID      log.Key = "app.category_id"
	KeyLoginMethod     log.Key = "app.login_method"
	KeyRegisterChannel log.Key = "app.register_channel"
	KeyCouponID        log.Key = "app.coupon_id"
)

// OrderPaidExtra 构造 order.paid 的 ExtraAttrs 示例。
func OrderPaidExtra(orderID, payChannel, paidAtRFC3339 string, amountCents int64) []slog.Attr {
	return []slog.Attr{
		slog.String(string(KeyOrderID), orderID),
		slog.Int64(string(KeyAmount), amountCents),
		slog.String(string(KeyPayChannel), payChannel),
		slog.String(string(KeyPaidAt), paidAtRFC3339),
	}
}
