package log

import "log/slog"

// 六类载荷：实现 EventPayload，属性键已按 semconv / app.* 对齐。

// RPCInfo 记录 RPC 调用的传输字段（semconv rpc）；gRPC 等 RPC 传输层使用，
// 与 HTTPInfo 互斥（按事件只填其一，零值省略）。
type RPCInfo struct {
	System  string // rpc.system：如 grpc
	Service string // rpc.service：如 mall.auth.v1.AuthService
	Method  string // rpc.method：如 Register
}

// AccessPayload 记录 HTTP/RPC 请求访问、状态、延迟与请求身份（每请求一条）。
type AccessPayload struct {
	EventName EventName
	Subject   Subject
	HTTP      HTTPInfo
	RPC       RPCInfo
	Result    Result
}

func (AccessPayload) EventType() EventType { return EventAccess }

func (e AccessPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 14)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyHTTPRequestMethod, e.HTTP.Method)
	attrs = appendString(attrs, KeyURLPath, e.HTTP.URLPath)
	attrs = appendString(attrs, KeyHTTPRoute, e.HTTP.Route)
	attrs = appendInt(attrs, KeyHTTPResponseStatusCode, e.HTTP.StatusCode)
	if e.HTTP.ClientIP != nil {
		attrs = appendString(attrs, KeyClientAddress, e.HTTP.ClientIP.String())
	}
	attrs = appendString(attrs, KeyUserAgentOriginal, e.HTTP.UserAgent)
	attrs = appendString(attrs, KeyRPCSystem, e.RPC.System)
	attrs = appendString(attrs, KeyRPCService, e.RPC.Service)
	attrs = appendString(attrs, KeyRPCMethod, e.RPC.Method)
	attrs = appendString(attrs, KeyUserID, e.Subject.UserID)
	attrs = appendString(attrs, KeyAppTenantID, e.Subject.TenantID)
	return appendString(attrs, KeyAppResult, string(e.Result))
}

// BusinessPayload 记录领域业务动作与业务拒绝。
// ErrorCode 业务错误码：（服务/模块）.（场景/操作）.（结果/具体错误）
type BusinessPayload struct {
	EventName EventName
	ErrorType string // error.type：business.* / validation.failed（低基数失败类别）
	Subject   Subject
	Resource  Resource
	ErrorCode string // app.error_code：稳定具体错误码。
	// BusinessCode 已弃用：请使用 ErrorCode；仅作为源码兼容输入，不再输出 app.business_code。
	BusinessCode    string
	BusinessMessage string
	Source          Source // code.function.name / code.file.path / code.line.number
	Result          Result
	// ExtraAttrs 是事件专属扩展字段：接入方按事件注入 app.* 键，
	// 随公共字段一起扁平输出；领域键由接入方自建（example/mall），核心 keys.go 不登记。
	// 与 BusinessPayload canonical 字段或公共保留字段重名的属性会被忽略。
	ExtraAttrs []slog.Attr
}

func (BusinessPayload) EventType() EventType { return EventBusiness }

func (e BusinessPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 14)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyErrorType, e.ErrorType)
	attrs = appendString(attrs, KeyUserID, e.Subject.UserID)
	attrs = appendString(attrs, KeyAppTenantID, e.Subject.TenantID)
	attrs = appendString(attrs, KeyAppResourceType, e.Resource.Type)
	attrs = appendString(attrs, KeyAppResourceID, e.Resource.ID)
	errorCode := e.ErrorCode
	if errorCode == "" {
		errorCode = e.BusinessCode
	}
	attrs = appendString(attrs, KeyAppErrorCode, errorCode)
	attrs = appendString(attrs, KeyAppBusinessMessage, e.BusinessMessage)
	attrs = appendString(attrs, KeyCodeFunctionName, e.Source.Function)
	attrs = appendString(attrs, KeyCodeFilePath, e.Source.Filepath)
	attrs = appendInt(attrs, KeyCodeLineNumber, e.Source.Line)
	for _, attr := range e.ExtraAttrs {
		if isBusinessExtraAttrKeyAllowed(attr.Key) {
			attrs = append(attrs, attr)
		}
	}
	return appendString(attrs, KeyAppResult, string(e.Result))
}

var businessPayloadCanonicalKeys = map[string]struct{}{
	string(KeyEventName):          {},
	string(KeyErrorType):          {},
	string(KeyUserID):             {},
	string(KeyAppUserID):          {},
	string(KeyAppTenantID):        {},
	string(KeyAppResourceType):    {},
	string(KeyAppResourceID):      {},
	string(KeyAppErrorCode):       {},
	string(KeyAppBusinessCode):    {},
	string(KeyAppBusinessMessage): {},
	string(KeyCodeFunctionName):   {},
	string(KeyCodeFilePath):       {},
	string(KeyCodeLineNumber):     {},
	string(KeyAppResult):          {},
}

func isBusinessExtraAttrKeyAllowed(key string) bool {
	if key == "" {
		return false
	}
	if _, canonical := businessPayloadCanonicalKeys[key]; canonical {
		return false
	}
	_, reserved := reservedKeys[key]
	return !reserved
}

// ErrorPayload 记录系统错误、panic、依赖失败或重试上下文。
// ErrorCode 业务错误码：（服务/模块）.（场景/操作）.（结果/具体错误）
type ErrorPayload struct {
	EventName           EventName
	ErrorType           string // error.type：低基数失败类别
	ErrorMessage        string // exception.message
	StackTrace          string // exception.stacktrace
	StackTraceTruncated bool   // app.stacktrace_truncated，仅在 StackTrace 非空时输出
	ErrorCode           string // app.error_code：可选业务关联码。
	// Operation 已弃用：请使用 ErrorCode；仅作为源码兼容输入，不再输出 app.operation。
	Operation        string
	FailureOperation string
	RootCauseType    string
	Retryable        bool
	RetryCount       int
	UpstreamService  string
	Source           Source
	Result           Result
	// ExtraAttrs 是事件专属扩展字段：接入方按事件注入 app.* 键（如非法输入摘要
	// app.input_*），随公共字段一起扁平输出；与 ErrorPayload canonical 字段或
	// 公共保留字段重名的属性会被忽略。
	ExtraAttrs []slog.Attr
}

func (ErrorPayload) EventType() EventType { return EventError }

func (e ErrorPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 16)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyErrorType, e.ErrorType)
	attrs = appendString(attrs, KeyExceptionMessage, e.ErrorMessage)
	attrs = appendString(attrs, KeyExceptionStacktrace, e.StackTrace)
	if e.StackTrace != "" {
		attrs = appendBool(attrs, KeyAppStacktraceTruncated, e.StackTraceTruncated)
	}
	errorCode := e.ErrorCode
	if errorCode == "" {
		errorCode = e.Operation
	}
	attrs = appendString(attrs, KeyAppErrorCode, errorCode)
	attrs = appendString(attrs, KeyAppFailureOperation, e.FailureOperation)
	attrs = appendString(attrs, KeyAppRootCauseType, e.RootCauseType)
	attrs = appendBool(attrs, KeyAppRetryable, e.Retryable)
	attrs = appendInt(attrs, KeyAppRetryCount, e.RetryCount)
	attrs = appendString(attrs, KeyAppUpstreamService, e.UpstreamService)
	attrs = appendString(attrs, KeyCodeFunctionName, e.Source.Function)
	attrs = appendString(attrs, KeyCodeFilePath, e.Source.Filepath)
	attrs = appendInt(attrs, KeyCodeLineNumber, e.Source.Line)
	for _, attr := range e.ExtraAttrs {
		if isErrorExtraAttrKeyAllowed(attr.Key) {
			attrs = append(attrs, attr)
		}
	}
	return appendString(attrs, KeyAppResult, string(e.Result))
}

// errorPayloadCanonicalKeys 是 ErrorPayload 的固定字段键：ExtraAttrs 不允许覆盖。
var errorPayloadCanonicalKeys = map[string]struct{}{
	string(KeyEventName):              {},
	string(KeyErrorType):              {},
	string(KeyExceptionMessage):       {},
	string(KeyExceptionStacktrace):    {},
	string(KeyAppStacktraceTruncated): {},
	string(KeyAppErrorCode):           {},
	string(KeyAppOperation):           {},
	string(KeyAppFailureOperation):    {},
	string(KeyAppRootCauseType):       {},
	string(KeyAppRetryable):           {},
	string(KeyAppRetryCount):          {},
	string(KeyAppUpstreamService):     {},
	string(KeyCodeFunctionName):       {},
	string(KeyCodeFilePath):           {},
	string(KeyCodeLineNumber):         {},
	string(KeyAppResult):              {},
}

// isErrorExtraAttrKeyAllowed 判断 ExtraAttrs 键是否允许注入 ErrorPayload：
// 空键、canonical 键与公共保留键一律忽略。
func isErrorExtraAttrKeyAllowed(key string) bool {
	if key == "" {
		return false
	}
	if _, canonical := errorPayloadCanonicalKeys[key]; canonical {
		return false
	}
	_, reserved := reservedKeys[key]
	return !reserved
}

// AuditPayload 记录高权限操作与敏感资源变更（审计，防篡改存储）。
type AuditPayload struct {
	EventName      EventName
	Action         string
	Actor          Actor
	Resource       Resource
	AuditEventType string
	TargetUserID   string
	ChangedFields  []string
	Before         Fields
	After          Fields
	Reason         string
	ApprovalID     string
	Result         Result
	// ExtraAttrs 是事件专属扩展字段：接入方按事件注入 app.* 键（如非法输入摘要
	// app.input_*），随公共字段一起扁平输出；与 AuditPayload canonical 字段或
	// 公共保留字段重名的属性会被忽略。
	ExtraAttrs []slog.Attr
}

func (AuditPayload) EventType() EventType { return EventAudit }

func (e AuditPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 14)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyAppAction, e.Action)
	attrs = appendString(attrs, KeyAppActorUserID, e.Actor.UserID)
	attrs = appendString(attrs, KeyAppActorRole, e.Actor.Role)
	attrs = appendString(attrs, KeyAppResourceType, e.Resource.Type)
	attrs = appendString(attrs, KeyAppResourceID, e.Resource.ID)
	attrs = appendString(attrs, KeyAppAuditEventType, e.AuditEventType)
	attrs = appendString(attrs, KeyAppTargetUserID, e.TargetUserID)
	attrs = appendSlice(attrs, KeyAppChangedFields, e.ChangedFields)
	attrs = appendFields(attrs, KeyAppBefore, e.Before)
	attrs = appendFields(attrs, KeyAppAfter, e.After)
	attrs = appendString(attrs, KeyAppReason, e.Reason)
	attrs = appendString(attrs, KeyAppApprovalID, e.ApprovalID)
	for _, attr := range e.ExtraAttrs {
		if isAuditExtraAttrKeyAllowed(attr.Key) {
			attrs = append(attrs, attr)
		}
	}
	return appendString(attrs, KeyAppResult, string(e.Result))
}

// auditPayloadCanonicalKeys 是 AuditPayload 的固定字段键：ExtraAttrs 不允许覆盖。
var auditPayloadCanonicalKeys = map[string]struct{}{
	string(KeyEventName):         {},
	string(KeyAppAction):         {},
	string(KeyAppActorUserID):    {},
	string(KeyAppActorRole):      {},
	string(KeyAppResourceType):   {},
	string(KeyAppResourceID):     {},
	string(KeyAppAuditEventType): {},
	string(KeyAppTargetUserID):   {},
	string(KeyAppChangedFields):  {},
	string(KeyAppBefore):         {},
	string(KeyAppAfter):          {},
	string(KeyAppReason):         {},
	string(KeyAppApprovalID):     {},
	string(KeyAppResult):         {},
}

// isAuditExtraAttrKeyAllowed 判断 ExtraAttrs 键是否允许注入 AuditPayload：
// 空键、canonical 键与公共保留键一律忽略。
func isAuditExtraAttrKeyAllowed(key string) bool {
	if key == "" {
		return false
	}
	if _, canonical := auditPayloadCanonicalKeys[key]; canonical {
		return false
	}
	_, reserved := reservedKeys[key]
	return !reserved
}

// SecurityPayload 记录认证、鉴权、风险或访问拦截行为（可直连 SIEM）。
type SecurityPayload struct {
	EventName         EventName
	Subject           Subject
	SecurityEventType string
	FailureReason     string
	ActionTaken       string
	RiskScore         int
	Result            Result
	// ExtraAttrs 是事件专属扩展字段：接入方按事件注入 app.* 键（如非法输入摘要
	// app.input_*），随公共字段一起扁平输出；与 SecurityPayload canonical 字段或
	// 公共保留字段重名的属性会被忽略。
	ExtraAttrs []slog.Attr
}

func (SecurityPayload) EventType() EventType { return EventSecurity }

func (e SecurityPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 9)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyUserID, e.Subject.UserID)
	attrs = appendString(attrs, KeyAppTenantID, e.Subject.TenantID)
	attrs = appendString(attrs, KeyAppSecurityEventType, e.SecurityEventType)
	attrs = appendString(attrs, KeyAppFailureReason, e.FailureReason)
	attrs = appendString(attrs, KeyAppActionTaken, e.ActionTaken)
	attrs = appendInt(attrs, KeyAppRiskScore, e.RiskScore)
	for _, attr := range e.ExtraAttrs {
		if isSecurityExtraAttrKeyAllowed(attr.Key) {
			attrs = append(attrs, attr)
		}
	}
	return appendString(attrs, KeyAppResult, string(e.Result))
}

// securityPayloadCanonicalKeys 是 SecurityPayload 的固定字段键：ExtraAttrs 不允许覆盖。
var securityPayloadCanonicalKeys = map[string]struct{}{
	string(KeyEventName):            {},
	string(KeyUserID):               {},
	string(KeyAppUserID):            {},
	string(KeyAppTenantID):          {},
	string(KeyAppSecurityEventType): {},
	string(KeyAppFailureReason):     {},
	string(KeyAppActionTaken):       {},
	string(KeyAppRiskScore):         {},
	string(KeyAppResult):            {},
}

// isSecurityExtraAttrKeyAllowed 判断 ExtraAttrs 键是否允许注入 SecurityPayload：
// 空键、canonical 键与公共保留键一律忽略。
func isSecurityExtraAttrKeyAllowed(key string) bool {
	if key == "" {
		return false
	}
	if _, canonical := securityPayloadCanonicalKeys[key]; canonical {
		return false
	}
	_, reserved := reservedKeys[key]
	return !reserved
}

// ProbePayload 记录健康、就绪、存活与启动探测结果。
type ProbePayload struct {
	EventName EventName
	HTTP      HTTPInfo
	ProbeType string
	ErrorCode string
	Source    string
	Result    Result
}

func (ProbePayload) EventType() EventType { return EventProbe }

func (e ProbePayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 8)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyHTTPRequestMethod, e.HTTP.Method)
	attrs = appendInt(attrs, KeyHTTPResponseStatusCode, e.HTTP.StatusCode)
	attrs = appendString(attrs, KeyAppProbeType, e.ProbeType)
	attrs = appendString(attrs, KeyAppErrorCode, e.ErrorCode)
	attrs = appendString(attrs, KeyAppSource, e.Source)
	return appendString(attrs, KeyAppResult, string(e.Result))
}
