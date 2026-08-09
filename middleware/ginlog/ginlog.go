// Package ginlog 提供 Gin 中间件：为每个请求生成 AccessEvent 并交给 log.Logger 写出。
// 采集频率仍由后端（otel-collector）控制；本中间件只负责事件组装与写出。
package ginlog

import (
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability"
)

// Config 中间件配置。
type Config struct {
	// Logger 必填：写出 access 事件的 Logger。
	Logger *log.Logger

	// GetRequestID 从请求提取 request_id；默认取 Header X-Request-ID，空则读 gin context 的 request_id。
	GetRequestID func(c *gin.Context) string

	// GetTraceContext 从请求/上下文提取 trace/span；默认读 gin context 的 trace_id / span_id。
	GetTraceContext func(c *gin.Context) log.TraceContext

	// GetUserID 提取用户标识；默认读 gin context 的 user_id。
	GetUserID func(c *gin.Context) string

	// SkipPaths 跳过不记录的路径（如健康检查）。
	SkipPaths map[string]bool

	// LevelForStatus 状态码映射缺省级别；默认 2xx-3xx=INFO、4xx=WARN、503=WARN、
	// 其余 5xx=ERROR（503 属暂时不可用、调用方可重试，记 WARN 而非 ERROR，
	// 避免重试噪音淹没告警）。
	LevelForStatus func(status int) log.Level

	// ResultForStatus 状态码映射业务结果；默认 >=400=failed，否则 success。
	ResultForStatus func(status int) log.Result
}

func defaultRequestID(c *gin.Context) string {
	if id := c.GetHeader("X-Request-ID"); id != "" {
		return id
	}
	return c.GetString("request_id")
}

func defaultTraceContext(c *gin.Context) log.TraceContext {
	// 优先取 OTel span（otelgin 中间件注入）；无有效 span 时回退到 gin context 值。
	sc := trace.SpanFromContext(c.Request.Context()).SpanContext()
	if sc.IsValid() {
		return log.TraceContext{TraceID: sc.TraceID().String(), SpanID: sc.SpanID().String()}
	}
	return log.TraceContext{TraceID: c.GetString("trace_id"), SpanID: c.GetString("span_id")}
}

func defaultUserID(c *gin.Context) string { return c.GetString("user_id") }

// defaultLevelForStatus 映射 HTTP 状态码的缺省级别：
// 2xx-3xx=INFO；4xx=WARN；503=WARN（暂时不可用、调用方可重试）；其余 5xx=ERROR。
// 调用方可通过 Config.LevelForStatus 整体覆盖。
func defaultLevelForStatus(status int) log.Level {
	switch {
	case status == http.StatusServiceUnavailable:
		return log.LevelWarn
	case status >= 500:
		return log.LevelError
	case status >= 400:
		return log.LevelWarn
	default:
		return log.LevelInfo
	}
}

func defaultResultForStatus(status int) log.Result {
	if status >= 400 {
		return log.ResultFailed
	}
	return log.ResultSuccess
}

// Middleware 返回记录 access 事件的 Gin 中间件。
// Logger 为空时 panic（配置错误应尽早暴露）。
func Middleware(cfg Config) gin.HandlerFunc {
	if cfg.Logger == nil {
		panic("ginlog: Logger 不能为空")
	}
	getRequestID := cfg.GetRequestID
	if getRequestID == nil {
		getRequestID = defaultRequestID
	}
	getTrace := cfg.GetTraceContext
	if getTrace == nil {
		getTrace = defaultTraceContext
	}
	getUserID := cfg.GetUserID
	if getUserID == nil {
		getUserID = defaultUserID
	}
	levelForStatus := cfg.LevelForStatus
	if levelForStatus == nil {
		levelForStatus = defaultLevelForStatus
	}
	resultForStatus := cfg.ResultForStatus
	if resultForStatus == nil {
		resultForStatus = defaultResultForStatus
	}

	return func(c *gin.Context) {
		if cfg.SkipPaths != nil && cfg.SkipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		tc := getTrace(c)
		ev := log.AccessEvent{
			EventMetadata: log.EventMetadata{
				Timestamp: start,
				Level:     levelForStatus(status),
				TraceID:   tc.TraceID,
				SpanID:    tc.SpanID,
				RequestID: getRequestID(c),
				LatencyMS: time.Since(start).Milliseconds(),
			},
			Data: log.AccessPayload{
				EventName: log.EventNameAccessHTTPRequest,
				Subject:   log.Subject{UserID: getUserID(c)},
				HTTP: log.HTTPInfo{
					Method:     c.Request.Method,
					Route:      c.FullPath(),
					URLPath:    c.Request.URL.Path,
					StatusCode: status,
					ClientIP:   parseIP(c.ClientIP()),
					UserAgent:  c.Request.UserAgent(),
				},
				Result: resultForStatus(status),
			},
		}
		cfg.Logger.Emit(c.Request.Context(), ev)
	}
}

func parseIP(s string) net.IP {
	if ip := net.ParseIP(s); ip != nil {
		return ip
	}
	return nil
}
