// Package mwutil 提供 middleware 各框架适配包共享的底层工具（内部包，不对外公开 API）。
// 只放「框架无关的机械工具」：HTTP 状态码记录、路由解析、gRPC 全方法解析与 span 收尾。
package mwutil

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// StatusRecorder 记录响应状态码（net/http 默认 200，WriteHeader 覆盖）。
type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

// WriteHeader 记录状态码并透传。
func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write 在未显式 WriteHeader 时按 200 兜底记录。
func (r *StatusRecorder) Write(data []byte) (int, error) {
	if r.Status == 0 {
		r.Status = http.StatusOK
	}
	return r.ResponseWriter.Write(data)
}

// HTTPRoute 返回路由模板；cfgRoute 为空时回退 net/http ServeMux 的 r.Pattern。
func HTTPRoute(cfgRoute func(r *http.Request) string, r *http.Request) string {
	if cfgRoute != nil {
		return cfgRoute(r)
	}
	return r.Pattern
}

// HTTPSpanName 生成初始 span name：method + target（gin 的 route 或 net/http 的 path）。
func HTTPSpanName(method, target string) string {
	if target == "" {
		return method
	}
	return method + " " + target
}

// HTTPSpanNameWithRoute 在路由已知后重命名 span：ServeMux 的 r.Pattern 自带 method
// （如 "GET /ok"），避免重复拼装。
func HTTPSpanNameWithRoute(method, route string) string {
	if route == "" {
		return method
	}
	if strings.HasPrefix(route, method) {
		return route
	}
	return method + " " + route
}

// FinishSpan 结束 span；handler panic 时标记错误后重抛，由外层 recover 中间件收口。
func FinishSpan(span trace.Span) {
	if recovered := recover(); recovered != nil {
		span.SetStatus(codes.Error, "panic")
		span.End()
		panic(recovered)
	}
	span.End()
}

// RPCService 从 gRPC FullMethod 提取 service 段（/pkg.Svc/Method → pkg.Svc）。
func RPCService(fullMethod string) string {
	parts := strings.Split(fullMethod, "/")
	if len(parts) >= 3 {
		return parts[1]
	}
	return ""
}

// RPCMethod 从 gRPC FullMethod 提取方法段（/pkg.Svc/Method → Method）。
func RPCMethod(fullMethod string) string {
	parts := strings.Split(fullMethod, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// Histogram 创建 Float64Histogram；创建失败即 panic（配置错误应尽早暴露）。
func Histogram(meter metric.Meter, name, description, unit string) metric.Float64Histogram {
	h, err := meter.Float64Histogram(name,
		metric.WithDescription(description), metric.WithUnit(unit))
	if err != nil {
		panic("mwutil: create histogram " + name + ": " + err.Error())
	}
	return h
}
