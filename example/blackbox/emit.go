package main

import (
	"context"
	"log/slog"
	"net"

	"github.com/formal-you/go-observability/errs"
	log "github.com/formal-you/go-observability/log"
)

// emitAll 生成覆盖 access / business / error 三类事件的日志，写入指定 Logger。
// 顺序固定：main.go 与 blackbox_test.go 共用，测试按行号逐行断言 JSON 结构。
func emitAll(ctx context.Context, l *log.Logger) {
	// 0: access——HTTP 请求成功
	l.Emit(ctx, log.AccessEvent{
		EventMetadata: log.EventMetadata{
			Level:     log.LevelInfo,
			TraceID:   "949058d5c20153624d52da3358038026",
			SpanID:    "bba473e9b3034af6",
			RequestID: "req-1001",
			LatencyMS: 12,
		},
		Data: log.AccessPayload{
			EventName: log.EventNameAccessHTTPRequest,
			HTTP: log.HTTPInfo{
				Method:     "GET",
				Route:      "/api/v1/products/:id",
				URLPath:    "/api/v1/products/42",
				StatusCode: 200,
				ClientIP:   net.ParseIP("127.0.0.1"),
				UserAgent:  "blackbox/1.0",
			},
			Result: log.ResultSuccess,
		},
	})

	// 1: access——HTTP 请求失败（404）
	l.Emit(ctx, log.AccessEvent{
		EventMetadata: log.EventMetadata{
			Level:     log.LevelWarn,
			TraceID:   "949058d5c20153624d52da3358038026",
			SpanID:    "c5e0c0b0a1b2c3d4",
			RequestID: "req-1002",
			LatencyMS: 3,
		},
		Data: log.AccessPayload{
			EventName: log.EventNameAccessHTTPRequest,
			HTTP: log.HTTPInfo{
				Method:     "GET",
				URLPath:    "/nope",
				StatusCode: 404,
				ClientIP:   net.ParseIP("127.0.0.1"),
				UserAgent:  "blackbox/1.0",
			},
			Result: log.ResultFailed,
		},
	})

	// 2: business——业务成功（订单支付）
	l.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("business", "order", "paid"),
			Result:    log.ResultSuccess,
			ExtraAttrs: []slog.Attr{
				slog.String("app.order_id", "ORD-20260811-0001"),
				slog.Float64("app.amount", 99.0),
			},
		},
	})

	// 3: business——业务拒绝（库存不足，三段式业务码）
	l.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelWarn},
		Data: log.BusinessPayload{
			EventName:       log.NewEventName("business", "order", "create"),
			ErrorType:       "business.order.stock_insufficient",
			BusinessCode:    "ORDER.CREATE.STOCK_INSUFFICIENT",
			BusinessMessage: "商品库存不足",
			Source:          log.Source{Function: "order.Create", Filepath: "internal/order/create.go", Line: 42},
			Result:          log.ResultBlocked,
		},
	})

	// 4: business——参数校验失败（errs.NewValidation → EventFromError 投影）
	l.Emit(ctx, log.EventFromError(
		log.NewEventName("business", "user", "register"),
		errs.NewValidation("用户名为空"),
		log.EventMetadata{},
	))

	// 5: error——非预期系统错误：DB 连接失败（StackMust → 自动采集堆栈；可重试未耗尽 → WARN）
	l.Emit(ctx, log.EventFromError(
		log.NewEventName("error", "db", "connection"),
		errs.NewSystem(errs.TypeDBConnectionError,
			"dial tcp 127.0.0.1:5432: connect: connection refused",
			errs.WithUpstream("postgres"),
			errs.WithRetry(1, false)),
		log.EventMetadata{},
	))

	// 6: error——MQ 发布失败（重试耗尽 → ERROR）
	l.Emit(ctx, log.EventFromError(
		log.NewEventName("error", "mq", "publish"),
		errs.NewSystem(errs.TypeMQPublishFailed,
			"publish order.created to topic order-events: deadline exceeded",
			errs.WithUpstream("kafka"),
			errs.WithRetry(3, true)),
		log.EventMetadata{},
	))

	// 7: error——运行时 panic（recover 场景，StackMust → 自动采集堆栈）
	l.Emit(ctx, log.EventFromError(
		log.EventNameErrorRuntimePanic,
		errs.NewSystem(errs.TypeRuntimePanic,
			"runtime error: invalid memory address or nil pointer dereference"),
		log.EventMetadata{},
	))

	// 8: error——锁冲突（StackOptional → 不自动采集堆栈，作为对比项）
	l.Emit(ctx, log.EventFromError(
		log.NewEventName("error", "lock", "conflict"),
		errs.NewSystem(errs.TypeLockConflict,
			"acquire lock order:pay:42 conflict"),
		log.EventMetadata{},
	))
}
