package log

import "log/slog"

// 六类载荷：实现 EventPayload，属性键已按 semconv / mall.* 对齐。

// AccessPayload 记录 HTTP 请求访问、状态、延迟与请求身份（每请求一条）。
type AccessPayload struct {
	EventName EventName
	Subject   Subject
	HTTP      HTTPInfo
	Result    Result
}

func (AccessPayload) EventType() EventType { return EventAccess }

func (e AccessPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 11)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyHTTPRequestMethod, e.HTTP.Method)
	attrs = appendString(attrs, KeyURLPath, e.HTTP.URLPath)
	attrs = appendString(attrs, KeyHTTPRoute, e.HTTP.Route)
	attrs = appendInt(attrs, KeyHTTPResponseStatusCode, e.HTTP.StatusCode)
	if e.HTTP.ClientIP != nil {
		attrs = appendString(attrs, KeyClientAddress, e.HTTP.ClientIP.String())
	}
	attrs = appendString(attrs, KeyUserAgentOriginal, e.HTTP.UserAgent)
	attrs = appendString(attrs, KeyMallUserID, e.Subject.UserID)
	attrs = appendString(attrs, KeyMallTenantID, e.Subject.TenantID)
	return appendString(attrs, KeyMallResult, string(e.Result))
}

// BusinessPayload 记录领域业务动作与业务拒绝。
type BusinessPayload struct {
	EventName       EventName
	ErrorType       string // error.type：business.* / validation.failed（低基数失败类别）
	Subject         Subject
	Resource        Resource
	BusinessCode    string // ErrCode：模块.场景.操作（格式校验随 B4 错误体系落地）
	BusinessMessage string
	Source          Source // code.function.name / code.file.path / code.line.number
	Result          Result
	// ExtraAttrs 事件专属扩展字段（B4 定稿）：业务侧按事件注入 mall.* 键，
	// 随公共字段一起扁平输出；键名登记 keys.go（AGENTS.md 规则 4）。
	ExtraAttrs []slog.Attr
}

func (BusinessPayload) EventType() EventType { return EventBusiness }

func (e BusinessPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 14)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyErrorType, e.ErrorType)
	attrs = appendString(attrs, KeyMallUserID, e.Subject.UserID)
	attrs = appendString(attrs, KeyMallTenantID, e.Subject.TenantID)
	attrs = appendString(attrs, KeyMallResourceType, e.Resource.Type)
	attrs = appendString(attrs, KeyMallResourceID, e.Resource.ID)
	attrs = appendString(attrs, KeyMallBusinessCode, e.BusinessCode)
	attrs = appendString(attrs, KeyMallBusinessMessage, e.BusinessMessage)
	attrs = appendString(attrs, KeyCodeFunctionName, e.Source.Function)
	attrs = appendString(attrs, KeyCodeFilePath, e.Source.Filepath)
	attrs = appendInt(attrs, KeyCodeLineNumber, e.Source.Line)
	attrs = append(attrs, e.ExtraAttrs...)
	return appendString(attrs, KeyMallResult, string(e.Result))
}

// ErrorPayload 记录系统错误、panic、依赖失败或重试上下文。
type ErrorPayload struct {
	EventName        EventName
	ErrorType        string // error.type：低基数失败类别
	ErrorMessage     string // exception.message
	StackTrace       string // exception.stacktrace
	Operation        string
	FailureOperation string
	RootCauseType    string
	Retryable        bool
	RetryCount       int
	UpstreamService  string
	Source           Source
	Result           Result
}

func (ErrorPayload) EventType() EventType { return EventError }

func (e ErrorPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 16)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyErrorType, e.ErrorType)
	attrs = appendString(attrs, KeyExceptionMessage, e.ErrorMessage)
	attrs = appendString(attrs, KeyExceptionStacktrace, e.StackTrace)
	attrs = appendString(attrs, KeyMallOperation, e.Operation)
	attrs = appendString(attrs, KeyMallFailureOperation, e.FailureOperation)
	attrs = appendString(attrs, KeyMallRootCauseType, e.RootCauseType)
	attrs = appendBool(attrs, KeyMallRetryable, e.Retryable)
	attrs = appendInt(attrs, KeyMallRetryCount, e.RetryCount)
	attrs = appendString(attrs, KeyMallUpstreamService, e.UpstreamService)
	attrs = appendString(attrs, KeyCodeFunctionName, e.Source.Function)
	attrs = appendString(attrs, KeyCodeFilePath, e.Source.Filepath)
	attrs = appendInt(attrs, KeyCodeLineNumber, e.Source.Line)
	return appendString(attrs, KeyMallResult, string(e.Result))
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
}

func (AuditPayload) EventType() EventType { return EventAudit }

func (e AuditPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 14)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyMallAction, e.Action)
	attrs = appendString(attrs, KeyMallActorUserID, e.Actor.UserID)
	attrs = appendString(attrs, KeyMallActorRole, e.Actor.Role)
	attrs = appendString(attrs, KeyMallResourceType, e.Resource.Type)
	attrs = appendString(attrs, KeyMallResourceID, e.Resource.ID)
	attrs = appendString(attrs, KeyMallAuditEventType, e.AuditEventType)
	attrs = appendString(attrs, KeyMallTargetUserID, e.TargetUserID)
	attrs = appendSlice(attrs, KeyMallChangedFields, e.ChangedFields)
	attrs = appendFields(attrs, KeyMallBefore, e.Before)
	attrs = appendFields(attrs, KeyMallAfter, e.After)
	attrs = appendString(attrs, KeyMallReason, e.Reason)
	attrs = appendString(attrs, KeyMallApprovalID, e.ApprovalID)
	return appendString(attrs, KeyMallResult, string(e.Result))
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
}

func (SecurityPayload) EventType() EventType { return EventSecurity }

func (e SecurityPayload) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 9)
	attrs = append(attrs, slog.String(string(KeyEventName), string(e.EventName)))
	attrs = appendString(attrs, KeyMallUserID, e.Subject.UserID)
	attrs = appendString(attrs, KeyMallTenantID, e.Subject.TenantID)
	attrs = appendString(attrs, KeyMallSecurityEventType, e.SecurityEventType)
	attrs = appendString(attrs, KeyMallFailureReason, e.FailureReason)
	attrs = appendString(attrs, KeyMallActionTaken, e.ActionTaken)
	attrs = appendInt(attrs, KeyMallRiskScore, e.RiskScore)
	return appendString(attrs, KeyMallResult, string(e.Result))
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
	attrs = appendString(attrs, KeyMallProbeType, e.ProbeType)
	attrs = appendString(attrs, KeyMallErrorCode, e.ErrorCode)
	attrs = appendString(attrs, KeyMallSource, e.Source)
	return appendString(attrs, KeyMallResult, string(e.Result))
}
