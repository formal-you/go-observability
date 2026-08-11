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

	// RPC（semconv rpc）
	KeyRPCSystem  Key = "rpc.system"
	KeyRPCService Key = "rpc.service"
	KeyRPCMethod  Key = "rpc.method"

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
	// 领域专属键（order_id/amount/…）不在核心登记：由接入方自建（example/mall）
	// 经 BusinessPayload.ExtraAttrs 注入，前缀仍使用 app.*。
)
