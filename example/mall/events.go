// Package mall 演示接入方如何自建业务事件注册表与专属键（C2 / B4）。
// 核心库只提供 BusinessPayload + ExtraAttrs + EventName 文法；领域名不进 types.go。
package mall

import (
	"log/slog"

	log "github.com/formal-you/go-observability/log"
)

// 业务事实名（<domain>.<subject>.<event>；粗分类由 BusinessEvent 的 type 承载）。
const (
	EventOrderCreated   log.EventName = "order.lifecycle.created"
	EventOrderPaid      log.EventName = "order.payment.succeeded"
	EventOrderCancelled log.EventName = "order.lifecycle.cancelled"
	EventRefundCreated  log.EventName = "refund.lifecycle.created"
	EventRefundSuccess  log.EventName = "refund.processing.succeeded"
	EventCartAdded      log.EventName = "cart.item.added"
	EventProductViewed  log.EventName = "product.detail.viewed"
	EventUserLogin      log.EventName = "user.session.started"
	EventUserRegistered log.EventName = "user.account.registered"
	EventCouponRedeemed log.EventName = "coupon.redemption.succeeded"
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
