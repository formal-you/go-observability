// Package mall 演示接入方如何自建 Event Registry（业务事件注册表）与专属键（C2 / B4）。
// 核心库只提供 BusinessPayload + ExtraAttrs + EventName 文法；领域名不进 types.go。
package mall

import (
	"log/slog"

	"github.com/formal-you/go-observability/errs"
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

// 错误事件名：错误收口中间件（ginmw.ErrorResponse）经 EventNameResolver 使用的领域事实名。
// 与 B4 业务事件清单分开：错误事件名承载“本次操作以何种事实失败/被拒绝”，
// 业务码到事件名的映射由接入方自建注册表维护，禁止在 resolver 里按 Kind 固定或手写。
const (
	// EventOrderCreateStockInsufficient 下单被库存不足拒绝（业务拒绝领域事件名）。
	EventOrderCreateStockInsufficient log.EventName = "order.create.stock_insufficient"
	// EventSystemError 未命中任何错误码注册表的系统错误兜底事实名。
	EventSystemError log.EventName = "system.error.occurred"
)

// businessErrorEvents 建立业务错误码到领域事件名的固定映射（接入方自建注册表）。
var businessErrorEvents = map[errs.ErrorCode]log.EventName{
	"ORDER.CREATE.STOCK_INSUFFICIENT": EventOrderCreateStockInsufficient,
}

// EventNameForErrorCode 返回业务错误码对应的领域错误事件名；未知码返回 false。
func EventNameForErrorCode(code errs.ErrorCode) (log.EventName, bool) {
	name, ok := businessErrorEvents[code]
	return name, ok
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
