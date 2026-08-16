// Command events 一次展示 log 包的六类类型化事件，以及 app.* 扩展字段如何进入 JSONL。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/02_events
//	Get-Content .\logs\events-six.jsonl
//
// 教学要点：
//   - 六类事件共享 EventMetadata，但载荷结构互不相同；
//   - 领域字段只能通过 ExtraAttrs 注入 app.* 键，核心库不登记具体业务键；
//   - 固定时间与 trace/span/request_id，便于课堂逐字段对照。
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"log/slog"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	ctx := context.Background()

	// file-only Runtime：不连接 Collector，直接写出本地 JSONL。
	runtime, err := telemetry.NewFileRuntime(telemetry.Config{
		Resource: telemetry.ResourceConfig{ServiceName: "events-demo", ServiceVersion: "0.1.0", Environment: "dev"},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/events-six.jsonl"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup telemetry:", err)
		os.Exit(1)
	}
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	w, err := runtime.NewLogWriter(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create writer:", err)
		os.Exit(1)
	}
	defer closeWriter(ctx, w)

	logger := log.NewLogger(w)

	// 固定时间与链路标识，让 JSONL 便于课堂对照与复制。
	ts := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	traceID := "0123456789abcdef0123456789abcdef"
	spanID := "0123456789abcdef"

	// access：每请求一条，语义键对齐 semconv http.*。
	logger.Emit(ctx, log.AccessEvent{
		EventMetadata: log.EventMetadata{Timestamp: ts, Level: log.LevelInfo, TraceID: traceID, SpanID: spanID, RequestID: "req-001", LatencyMS: 12},
		Data: log.AccessPayload{
			EventName: log.EventNameHTTPRequestCompleted,
			Subject:   log.Subject{UserID: "u_1001"},
			HTTP: log.HTTPInfo{
				Method:     "GET",
				Route:      "/api/v1/products/:id",
				URLPath:    "/api/v1/products/42",
				StatusCode: 200,
				ClientIP:   net.ParseIP("127.0.0.1"),
				UserAgent:  "teaching/1.0",
			},
			Result: log.ResultSuccess,
		},
	})

	// business：领域动作/业务拒绝。ExtraAttrs 是唯一合法的领域扩展出口。
	logger.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Timestamp: ts, Level: log.LevelInfo, TraceID: traceID, SpanID: spanID, RequestID: "req-002"},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("order", "payment", "succeeded"),
			Subject:   log.Subject{UserID: "u_1001"},
			Resource:  log.Resource{Type: "order", ID: "ORD-1001"},
			Result:    log.ResultSuccess,
			ExtraAttrs: []slog.Attr{
				slog.String("app.order_id", "ORD-1001"),
				slog.Int64("app.amount_cents", 9900),
				slog.String("app.pay_channel", "wechat"),
			},
		},
	})

	// error：系统错误、依赖失败或重试上下文；ErrorType 是低基数失败类别。
	logger.Emit(ctx, log.ErrorEvent{
		EventMetadata: log.EventMetadata{Timestamp: ts, Level: log.LevelError, TraceID: traceID, SpanID: spanID, RequestID: "req-002"},
		Data: log.ErrorPayload{
			EventName:       log.NewEventName("order", "payment", "gateway_timeout"),
			ErrorType:       "DEADLINE_EXCEEDED",
			ErrorMessage:    "payment gateway timeout",
			ErrorCode:       "PAY.GATEWAY.TIMEOUT",
			Retryable:       true,
			RetryCount:      2,
			UpstreamService: "payment-gateway",
			Result:          log.ResultError,
		},
	})

	// security：认证/鉴权/风控拦截；Result 使用 denied 等高价值结果，便于保留策略。
	logger.Emit(ctx, log.SecurityEvent{
		EventMetadata: log.EventMetadata{Timestamp: ts, Level: log.LevelWarn, TraceID: traceID, SpanID: spanID, RequestID: "req-003"},
		Data: log.SecurityPayload{
			EventName:         log.NewEventName("auth", "login", "denied"),
			Subject:           log.Subject{UserID: "u_1001"},
			SecurityEventType: "auth.failure",
			FailureReason:     "password_mismatch",
			ActionTaken:       "blocked",
			RiskScore:         70,
			Result:            log.ResultDenied,
		},
	})

	// audit：高权限操作与敏感资源变更；Before/After 用于追溯字段级变化。
	logger.Emit(ctx, log.AuditEvent{
		EventMetadata: log.EventMetadata{Timestamp: ts, Level: log.LevelInfo, TraceID: traceID, SpanID: spanID, RequestID: "req-004"},
		Data: log.AuditPayload{
			EventName:      log.NewEventName("admin", "role_update", "recorded"),
			Action:         "admin.role_update",
			Actor:          log.Actor{UserID: "u_admin", Role: "admin"},
			Resource:       log.Resource{Type: "user", ID: "u-2002"},
			AuditEventType: "admin.operation",
			TargetUserID:   "u-2002",
			ChangedFields:  []string{"role"},
			Before:         log.Fields{"role": "member"},
			After:          log.Fields{"role": "operator"},
			Reason:         "授权变更",
			Result:         log.ResultSuccess,
		},
	})

	// probe：健康/就绪/存活探测；与 access 分开建模，便于确定性排除探针噪音。
	logger.Emit(ctx, log.ProbeEvent{
		EventMetadata: log.EventMetadata{Timestamp: ts, Level: log.LevelInfo, TraceID: traceID, SpanID: spanID},
		Data: log.ProbePayload{
			EventName: log.NewEventName("health", "check", "completed"),
			HTTP:      log.HTTPInfo{Method: "GET", URLPath: "/healthz", StatusCode: 200},
			ProbeType: "liveness",
			Result:    log.ResultSuccess,
		},
	})

	fmt.Println("written: logs/events-six.jsonl")
}

// closeWriter 在函数返回前释放 Writer 自己拥有的资源。
func closeWriter(ctx context.Context, w log.ManagedWriter) {
	_ = w.Close(ctx)
}
