package log

// Key 属性键类型：镜像 OTel semconv 名称，保持核心包零 OTel 依赖（纯字符串常量）。
// 标准字段必须与 semconv 完全一致；业务字段使用 app.* 命名空间避免冲突。
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
	KeyAppUserID            Key = "app.user_id"
	KeyAppTenantID          Key = "app.tenant_id"
	KeyAppResult            Key = "app.result"
	KeyAppBusinessCode      Key = "app.business_code"
	KeyAppBusinessMessage   Key = "app.business_message"
	KeyAppResourceType      Key = "app.resource_type"
	KeyAppResourceID        Key = "app.resource_id"
	KeyAppOperation         Key = "app.operation"
	KeyAppFailureOperation  Key = "app.failure_operation"
	KeyAppRootCauseType     Key = "app.root_cause_type"
	KeyAppRetryable         Key = "app.retryable"
	KeyAppRetryCount        Key = "app.retry_count"
	KeyAppUpstreamService   Key = "app.upstream_service"
	KeyAppAction            Key = "app.action"
	KeyAppActorUserID       Key = "app.actor_user_id"
	KeyAppActorRole         Key = "app.actor_role"
	KeyAppAuditEventType    Key = "app.audit_event_type"
	KeyAppTargetUserID      Key = "app.target_user_id"
	KeyAppChangedFields     Key = "app.changed_fields"
	KeyAppBefore            Key = "app.before"
	KeyAppAfter             Key = "app.after"
	KeyAppReason            Key = "app.reason"
	KeyAppApprovalID        Key = "app.approval_id"
	KeyAppSecurityEventType Key = "app.security_event_type"
	KeyAppFailureReason     Key = "app.failure_reason"
	KeyAppActionTaken       Key = "app.action_taken"
	KeyAppRiskScore         Key = "app.risk_score"
	KeyAppProbeType         Key = "app.probe_type"
	KeyAppErrorCode         Key = "app.error_code"
	KeyAppSource            Key = "app.source"

	// 业务事件专属字段（B4 定稿：app.<snake_case>，随 ExtraAttrs 注入）
	KeyAppOrderID         Key = "app.order_id"
	KeyAppAmount          Key = "app.amount"
	KeyAppItemCount       Key = "app.item_count"
	KeyAppChannel         Key = "app.channel"
	KeyAppPayChannel      Key = "app.pay_channel"
	KeyAppPaidAt          Key = "app.paid_at"
	KeyAppCancelledAt     Key = "app.cancelled_at"
	KeyAppRefundID        Key = "app.refund_id"
	KeyAppRefundedAt      Key = "app.refunded_at"
	KeyAppProductID       Key = "app.product_id"
	KeyAppQuantity        Key = "app.quantity"
	KeyAppCategoryID      Key = "app.category_id"
	KeyAppLoginMethod     Key = "app.login_method"
	KeyAppRegisterChannel Key = "app.register_channel"
	KeyAppCouponID        Key = "app.coupon_id"
)
