package log

// Key 属性键类型：镜像 OTel semconv 名称，保持核心包零 OTel 依赖（纯字符串常量）。
// 标准字段必须与 semconv 完全一致；业务字段使用 mall.* 命名空间避免冲突。
type Key string

const (
	// 事件（semconv general/events）
	// event.name 仅作为传输键：OTLP 路径由 attrkv 映射到 LogRecord 的 EventName 顶层字段，
	// file/stdout 扁平投影保留 event.name 键（1.41.0 的属性名 otel.event.name 只用于桥接场景）。
	KeyEventName Key = "event.name"
	KeyTimestamp Key = "timestamp"
	KeyLevel     Key = "level"

	// HTTP（semconv http；路径用 url.path，1.41.0 已移除 http.request.path）
	KeyHTTPRequestMethod      Key = "http.request.method"
	KeyURLPath                Key = "url.path"
	KeyHTTPRoute              Key = "http.route"
	KeyHTTPResponseStatusCode Key = "http.response.status_code"
	KeyClientAddress          Key = "client.address"
	KeyUserAgentOriginal      Key = "user_agent.original"

	// 错误（semconv error / exception）
	KeyErrorType           Key = "error.type"
	KeyExceptionType       Key = "exception.type"
	KeyExceptionMessage    Key = "exception.message"
	KeyExceptionStacktrace Key = "exception.stacktrace"

	// 代码位置（semconv code：1.41.0 命名为 code.function.name / code.file.path / code.line.number）
	KeyCodeFunctionName Key = "code.function.name"
	KeyCodeFilePath     Key = "code.file.path"
	KeyCodeLineNumber   Key = "code.line.number"

	// 通用（关联链路：由 EventMetadata 输出，Writer 再转 OTLP TraceId/SpanId 字节）
	KeyTraceID Key = "trace_id"
	KeySpanID  Key = "span_id"

	// 通用 vendor
	KeyRequestID Key = "request_id"
	KeyLatencyMS Key = "latency_ms"

	// 业务 vendor 命名空间
	KeyMallUserID            Key = "mall.user_id"
	KeyMallTenantID          Key = "mall.tenant_id"
	KeyMallResult            Key = "mall.result"
	KeyMallBusinessCode      Key = "mall.business_code"
	KeyMallBusinessMessage   Key = "mall.business_message"
	KeyMallResourceType      Key = "mall.resource_type"
	KeyMallResourceID        Key = "mall.resource_id"
	KeyMallOperation         Key = "mall.operation"
	KeyMallFailureOperation  Key = "mall.failure_operation"
	KeyMallRootCauseType     Key = "mall.root_cause_type"
	KeyMallRetryable         Key = "mall.retryable"
	KeyMallRetryCount        Key = "mall.retry_count"
	KeyMallUpstreamService   Key = "mall.upstream_service"
	KeyMallAction            Key = "mall.action"
	KeyMallActorUserID       Key = "mall.actor_user_id"
	KeyMallActorRole         Key = "mall.actor_role"
	KeyMallAuditEventType    Key = "mall.audit_event_type"
	KeyMallTargetUserID      Key = "mall.target_user_id"
	KeyMallChangedFields     Key = "mall.changed_fields"
	KeyMallBefore            Key = "mall.before"
	KeyMallAfter             Key = "mall.after"
	KeyMallReason            Key = "mall.reason"
	KeyMallApprovalID        Key = "mall.approval_id"
	KeyMallSecurityEventType Key = "mall.security_event_type"
	KeyMallFailureReason     Key = "mall.failure_reason"
	KeyMallActionTaken       Key = "mall.action_taken"
	KeyMallRiskScore         Key = "mall.risk_score"
	KeyMallProbeType         Key = "mall.probe_type"
	KeyMallErrorCode         Key = "mall.error_code"
	KeyMallSource            Key = "mall.source"

	// 业务事件专属字段（B4 定稿：mall.<snake_case>，随 ExtraAttrs 注入）
	KeyMallOrderID         Key = "mall.order_id"
	KeyMallAmount          Key = "mall.amount"
	KeyMallItemCount       Key = "mall.item_count"
	KeyMallChannel         Key = "mall.channel"
	KeyMallPayChannel      Key = "mall.pay_channel"
	KeyMallPaidAt          Key = "mall.paid_at"
	KeyMallCancelledAt     Key = "mall.cancelled_at"
	KeyMallRefundID        Key = "mall.refund_id"
	KeyMallRefundedAt      Key = "mall.refunded_at"
	KeyMallProductID       Key = "mall.product_id"
	KeyMallQuantity        Key = "mall.quantity"
	KeyMallCategoryID      Key = "mall.category_id"
	KeyMallLoginMethod     Key = "mall.login_method"
	KeyMallRegisterChannel Key = "mall.register_channel"
	KeyMallCouponID        Key = "mall.coupon_id"
)
