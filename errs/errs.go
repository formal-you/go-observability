// Package errs 定义服务端应用的统一错误体系：错误分类（ErrorKind）、低基数失败类别
// （ErrorType）、业务错误码（ErrorCode）、代码位置（Source）与堆栈策略（StackPolicy）。
//
// 职责边界：本包只负责错误值的建模与构造，不负责记录日志、上报 OTel 或渲染响应；
// 收口层（middleware）通过 AppError 接口读取 Kind/ErrCode/ErrorType 做统一判定与
// 渲染；StackMust 类别的堆栈在 NewSystem 构造点自动采集，收口层只渲染、不重复采集。
// 依赖方向：本包只依赖标准库（errors、runtime、runtime/debug、strings），不依赖任何
// 外部日志/观测库，可被任意层安全引用。
package errs

import (
	"errors"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

// ErrorKind 错误预期性分类：validation / business / system。
// 收口层据此区分「预期内拒绝」（validation/business，正常响应路径，不告警）与
// 「非预期故障」（system，需要告警与重试评估）。
type ErrorKind string

const (
	// KindValidation 参数/入参校验失败，属预期内拒绝。
	KindValidation ErrorKind = "validation"
	// KindBusiness 业务规则拒绝（如库存不足、幂等冲突），属预期内拒绝。
	KindBusiness ErrorKind = "business"
	// KindSystem 系统非预期故障（DB/Redis/MQ/HTTP 上游等），需要告警与重试评估。
	KindSystem ErrorKind = "system"
)

// ErrorType 映射 OTel error.type：保持低基数，采用 domain.reason 格式
// （第一段为资源域 db/redis/mq/http/lock/idempotency/stock/data/runtime/validation/
// business 等，后接具体失败原因，如 db.connection_error / business.stock_insufficient）。
// 它是比 ErrorCode 更高层的全局分类：不针对某个具体业务码细化，而是把多个
// ErrorCode 按失效模式归为同一大类（多对一），用于聚合、告警路由与堆栈策略。
// 系统侧为固定枚举（本文件下方常量），business.* 为开放命名空间，本包不穷举，
// 调用方可用 ErrorType("business.stock_insufficient") 或自定义常量。
type ErrorType string

const (
	// TypeUnknown 普通 error 无法归类时的稳定兜底类型。
	TypeUnknown ErrorType = "error.unknown"
	// TypeValidationFailed 参数/入参校验失败。
	TypeValidationFailed ErrorType = "validation.failed"
	// TypeDBConnectionError 数据库连接失败。
	TypeDBConnectionError ErrorType = "db.connection_error"
	// TypeDBQueryTimeout 数据库查询超时。
	TypeDBQueryTimeout ErrorType = "db.query_timeout"
	// TypeDBDeadlock 数据库死锁。
	TypeDBDeadlock ErrorType = "db.deadlock"
	// TypeRedisConnectionError Redis 连接失败。
	TypeRedisConnectionError ErrorType = "redis.connection_error"
	// TypeRedisTimeout Redis 操作超时。
	TypeRedisTimeout ErrorType = "redis.timeout"
	// TypeMQPublishFailed 消息发布失败。
	TypeMQPublishFailed ErrorType = "mq.publish_failed"
	// TypeMQConsumeFailed 消息消费失败。
	TypeMQConsumeFailed ErrorType = "mq.consume_failed"
	// TypeMQRetryExhausted 消息重试耗尽。
	TypeMQRetryExhausted ErrorType = "mq.retry_exhausted"
	// TypeHTTPUpstream5xx 上游 HTTP 服务返回 5xx。
	TypeHTTPUpstream5xx ErrorType = "http.upstream_5xx"
	// TypeHTTPUpstreamTimeout 上游 HTTP 服务调用超时。
	TypeHTTPUpstreamTimeout ErrorType = "http.upstream_timeout"
	// TypeLockConflict 分布式锁冲突。
	TypeLockConflict ErrorType = "lock.conflict"
	// TypeIdempotencyConflict 幂等冲突（重复请求被拒绝）。
	TypeIdempotencyConflict ErrorType = "idempotency.conflict"
	// TypeStockRace 库存并发竞争。
	TypeStockRace ErrorType = "stock.race"
	// TypeDataJSONUnmarshal JSON 反序列化失败。
	TypeDataJSONUnmarshal ErrorType = "data.json_unmarshal"
	// TypeDataDuplicateKey 数据唯一键冲突。
	TypeDataDuplicateKey ErrorType = "data.duplicate_key"
	// TypeDataNotFound 数据不存在。
	TypeDataNotFound ErrorType = "data.not_found"
	// TypeRuntimePanic 运行时 panic。
	TypeRuntimePanic ErrorType = "runtime.panic"
	// TypeRuntimeContextCanceled 上下文被取消。
	TypeRuntimeContextCanceled ErrorType = "runtime.context_cancelled"
	// TypeRuntimeDeadlineExceeded 操作超过截止时间。
	TypeRuntimeDeadlineExceeded ErrorType = "runtime.deadline_exceeded"
)

// ErrorCode 业务错误码：（服务/模块）.（场景/操作）.（结果/具体错误）
// （如 ORDER.CREATE.STOCK_INSUFFICIENT）。
// 仅 BizError 承载；SystemError 可通过 WithCode 关联可选业务码。
type ErrorCode string

// Source 代码位置（OTel code.* 语义），用于定位错误产生点。
type Source struct {
	Function string // code.function.name
	Filepath string // code.file.path
	Line     int    // code.line.number
}

// StackPolicy 堆栈策略：must=构造点必记 / optional=按需记录 / none=不记。
// StackMust 由 NewSystem 构造时自动调用 CaptureStack；optional/none 不自动采集。
type StackPolicy string

const (
	// StackMust 必记创建点堆栈：跨进程/易失性失败（runtime/db/redis/mq/http），
	// 由 NewSystem 构造时自动采集。
	StackMust StackPolicy = "must"
	// StackOptional 按需记录堆栈：低基数但可复现的失败（lock/idempotency/stock/data）
	// 与高频的 runtime.context_cancelled；不自动采集，需要时由调用方 WithStack。
	StackOptional StackPolicy = "optional"
	// StackNone 不记录堆栈：预期内的业务/校验拒绝，堆栈无诊断价值。
	StackNone StackPolicy = "none"
)

// stackOverrides 保存使用方注入的「error.type 前缀 → 堆栈策略」覆盖表；命中时优先于
// 内置默认策略。由 SetStackPolicy 在进程启动期一次性写入，之后只读。
var (
	stackOverridesMu sync.RWMutex
	stackOverrides   = map[string]StackPolicy{}
)

// SetStackPolicy 设置前缀→策略覆盖表（如 "db." -> StackNone、精确类型
// "runtime.context_cancelled" -> StackMust）；空 map / nil 恢复为仅使用内置默认策略。
// 必须在进程启动阶段、任何错误构造/事件写出前调用（与 log.SetFlags 同类）；
// StackRule 对命中覆盖前缀的类型优先返回覆盖策略（最长前缀优先），未命中回落到内置默认。
func SetStackPolicy(overrides map[string]StackPolicy) {
	copied := make(map[string]StackPolicy, len(overrides))
	for prefix, policy := range overrides {
		if prefix == "" {
			continue
		}
		copied[prefix] = policy
	}
	stackOverridesMu.Lock()
	stackOverrides = copied
	stackOverridesMu.Unlock()
}

// StackRule 按 error.type 返回堆栈策略（NewSystem 构造时据此决定是否自动补记创建点堆栈）：
// 先查 SetStackPolicy 注入的覆盖表（最长前缀优先），未命中回落到内置默认——
// db./redis./mq./http./runtime.（除 context_cancelled）-> must；
// lock./idempotency./stock.race/data. 及 runtime.context_cancelled -> optional；
// business.*/validation.* 及未知/空类型（默认兜底）-> none。
func StackRule(t ErrorType) StackPolicy {
	p := string(t)
	stackOverridesMu.RLock()
	policy, ok := stackPolicyForLocked(p)
	stackOverridesMu.RUnlock()
	if ok {
		return policy
	}
	switch {
	case p == "runtime.context_cancelled":
		// 高频且多为客户端主动取消，逐次采集堆栈成本高、诊断价值低；需要时由调用方 WithStack。
		return StackOptional
	case hasAnyPrefix(p, "runtime.", "db.", "redis.", "mq.", "http."):
		return StackMust
	case hasAnyPrefix(p, "lock.", "idempotency.", "stock.race", "data."):
		return StackOptional
	default:
		return StackNone
	}
}

// stackPolicyForLocked 在覆盖表中查找 p 的最长匹配前缀；调用方须持有 stackOverridesMu 读锁。
func stackPolicyForLocked(p string) (StackPolicy, bool) {
	best := ""
	var bestPolicy StackPolicy
	found := false
	for prefix, policy := range stackOverrides {
		if len(prefix) > len(best) && strings.HasPrefix(p, prefix) {
			best = prefix
			bestPolicy = policy
			found = true
		}
	}
	return bestPolicy, found
}

// hasAnyPrefix 报告 s 是否以 prefixes 中任一前缀开头。
// StackRule 内部复用，避免在每个分支重复书写 HasPrefix 判断。
func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// AppError 错误体系接口：统一收口所需方法。
// 收口层（middleware）通过该接口读取 Kind/ErrCode/ErrorType 做分类与响应映射，
// 通过 Unwrap 保留 cause 链；需要进入统一收口的自定义错误须实现本接口。
type AppError interface {
	error
	Kind() ErrorKind
	ErrCode() ErrorCode
	ErrorType() ErrorType
	Unwrap() error
}

// appError 错误基类（未导出）：持有消息与 cause，提供零值默认实现；
// 未知/未设置字段按 KindSystem 兜底，保证零值也能作为 error 使用。
type appError struct {
	message string
	cause   error
}

// Error 返回错误消息；存在 cause 时以「消息: cause」拼接，保持错误链可读。
func (e appError) Error() string {
	if e.cause != nil {
		return e.message + ": " + e.cause.Error()
	}
	return e.message
}

// Kind 返回 KindSystem：基类不区分业务预期性，未知错误一律按系统错误兜底。
func (appError) Kind() ErrorKind {
	return KindSystem
}

// ErrCode 返回空 ErrorCode：基类不承载业务错误码。
func (appError) ErrCode() ErrorCode {
	return ""
}

// ErrorType 返回空 ErrorType：基类不预设失败类别。
func (appError) ErrorType() ErrorType {
	return ""
}

// Unwrap 返回 cause，供 errors.Is/errors.As 沿错误链查找根因。
func (e appError) Unwrap() error {
	return e.cause
}

// BizError 业务预期拒绝：validation/business 共用，Kind 区分。
// 业务层返回预期内失败（参数校验不过、库存不足、幂等冲突等）时使用本类型；
// 收口层按 Kind 映射 HTTP 状态码与是否告警，不把预期拒绝当作系统故障。
type BizError struct {
	appError
	code   ErrorCode
	typ    ErrorType
	kind   ErrorKind
	source Source
}

// Kind 返回业务预期性分类（KindValidation 或 KindBusiness）。
func (e BizError) Kind() ErrorKind {
	return e.kind
}

// ErrCode 返回业务错误码；参数校验错误（NewValidation）返回空。
func (e BizError) ErrCode() ErrorCode {
	return e.code
}

// ErrorType 返回低基数失败类别（如 validation.failed 或 business.*）。
func (e BizError) ErrorType() ErrorType {
	return e.typ
}

// Source 返回代码位置；未设置时为零值 Source{}。
func (e BizError) Source() Source {
	return e.source
}

// NewValidation 参数校验错误：typ=TypeValidationFailed，不记 ErrCode。
// 调用方只需传可读 message，分类与类别由本函数固定，避免业务层误填。
func NewValidation(message string) BizError {
	return BizError{
		appError: appError{message: message},
		code:     "",
		typ:      TypeValidationFailed,
		kind:     KindValidation,
	}
}

// NewBusiness 业务拒绝：ErrCode 必填，typ 为 business.* 类别
// （如 TypeBusinessStockInsufficient 由调用方传 string 转 ErrorType 或自定义常量）。
func NewBusiness(code ErrorCode, typ ErrorType, message string) BizError {
	return BizError{
		appError: appError{message: message},
		code:     code,
		typ:      typ,
		kind:     KindBusiness,
	}
}

// WithSource 返回带 source 的副本（不可变风格）：不修改原值，链式调用时原错误可复用。
func (e BizError) WithSource(s Source) BizError {
	e.source = s
	return e
}

// SystemError 系统错误：error.type 必填；StackMust 类别的堆栈在构造时自动补记
// （创建点），可选重试上下文。
// 非预期失败（DB/Redis/MQ/HTTP 上游/运行时）使用本类型，收口层据此告警并按类型映射。
type SystemError struct {
	appError
	typ       ErrorType
	code      ErrorCode
	retryable bool
	retries   int
	exhausted bool
	upstream  string
	stack     string
	source    Source
}

// Kind 返回 KindSystem：系统错误一律属于系统分类。
func (e SystemError) Kind() ErrorKind {
	return KindSystem
}

// ErrCode 返回可选业务关联码；未设置时为空。
func (e SystemError) ErrCode() ErrorCode {
	return e.code
}

// ErrorType 返回低基数失败类别（db./redis./http. 等），必填。
func (e SystemError) ErrorType() ErrorType {
	return e.typ
}

// Retryable 返回是否可重试（由 WithRetry 设置）。
func (e SystemError) Retryable() bool {
	return e.retryable
}

// Retries 返回已重试次数（由 WithRetry 设置）。
func (e SystemError) Retries() int {
	return e.retries
}

// RetriesExhausted 返回是否重试耗尽（由 WithRetry 设置）。
func (e SystemError) RetriesExhausted() bool {
	return e.exhausted
}

// Upstream 返回上游服务名（由 WithUpstream 设置）。
func (e SystemError) Upstream() string {
	return e.upstream
}

// Stack 返回收口层填充的堆栈（由 WithStack 设置）。
func (e SystemError) Stack() string {
	return e.stack
}

// Source 返回代码位置；未设置时为零值 Source{}。
func (e SystemError) Source() Source {
	return e.source
}

// SystemOption 构造选项：函数式选项，修改 *SystemError，NewSystem 按传入顺序应用。
type SystemOption func(*SystemError)

// WithRetry 设置重试上下文：retryable=true，retries=已重试次数，exhausted=是否耗尽。
func WithRetry(retries int, exhausted bool) SystemOption {
	return func(e *SystemError) {
		e.retryable = true
		e.retries = retries
		e.exhausted = exhausted
	}
}

// WithUpstream 设置上游服务名。
func WithUpstream(name string) SystemOption {
	return func(e *SystemError) {
		e.upstream = name
	}
}

// WithCode 设置可选业务关联码。
func WithCode(code ErrorCode) SystemOption {
	return func(e *SystemError) {
		e.code = code
	}
}

// WithStack 设置堆栈；显式传入时优先于 StackMust 的自动采集。
func WithStack(stack string) SystemOption {
	return func(e *SystemError) {
		e.stack = stack
	}
}

// WithSource 设置代码位置。
func WithSource(s Source) SystemOption {
	return func(e *SystemError) {
		e.source = s
	}
}

// NewSystem 构造系统错误：typ 必填（低基数失败类别），message 为可读描述；
// 可选选项按传入顺序生效（重试上下文、上游、关联码、堆栈、代码位置）。
// StackMust 类别在构造点自动补记创建点堆栈（诊断价值高于收口点采集）；
// 显式 WithStack 优先，避免覆盖调用方提供的堆栈。
func NewSystem(typ ErrorType, message string, opts ...SystemOption) SystemError {
	e := SystemError{
		appError: appError{message: message},
		typ:      typ,
	}
	for _, opt := range opts {
		opt(&e)
	}
	// StackMust 类别构造时自动补记堆栈；StackOptional/StackNone 不自动采集。
	if e.stack == "" && StackRule(e.typ) == StackMust {
		e.stack = CaptureStack()
	}
	return e
}

// CaptureSource 捕获调用点代码位置（skip 从 CaptureSource 自身开始计，建议调用方传 1-2）。
// 用于填充 Source 并映射 OTel code.function.name / code.file.path / code.line.number；
// 内部跳过 runtime.* 帧，保证返回的是第一个业务帧。
func CaptureSource(skip int) Source {
	// runtime.Callers 的 skip 从自身开始计，+1 把 CaptureSource 计入 skip 偏移。
	var pcs [16]uintptr
	n := runtime.Callers(skip+1, pcs[:])
	if n == 0 {
		return Source{}
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if !strings.HasPrefix(f.Function, "runtime.") {
			return Source{
				Function: f.Function,
				Filepath: f.File,
				Line:     f.Line,
			}
		}
		if !more {
			return Source{}
		}
	}
}

// CaptureStack 捕获当前 goroutine 完整堆栈（runtime/debug.Stack 的字符串形式，供 StackMust 场景）。
// NewSystem 对 StackMust 类别自动调用（构造点采集）；StackOptional/StackNone 不自动
// 采集，需要时由调用方显式 WithStack。
func CaptureStack() string {
	return string(debug.Stack())
}

// IsRetryExhausted 判断错误是否重试耗尽（SystemError 且 exhausted；BizError/其他返回 false）。
// 沿错误链使用 errors.As 查找，可识别被外层 %w 包裹的 SystemError。
func IsRetryExhausted(err error) bool {
	var exhausted interface {
		RetriesExhausted() bool
	}
	if errors.As(err, &exhausted) {
		return exhausted.RetriesExhausted()
	}
	return false
}
