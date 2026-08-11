package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/errs"
	log "github.com/formal-you/go-observability/log"
	ginmw "github.com/formal-you/go-observability/middleware/gin"
	"github.com/formal-you/go-observability/middleware/httperr"
)

const (
	requestBusinessSuccess = "req-business-success"
	requestBusinessFailed  = "req-business-failed"
	requestSystemError     = "req-system-error"
	requestPanic           = "req-panic"
)

// scenarioTrace 保存一次黑盒场景的真实 OTel span 标识，供人工联调和测试关联日志。
type scenarioTrace struct {
	TraceID string
	SpanID  string
}

// scenarioReport 汇总场景对应的 trace/span；请求按 request_id、后台任务按场景名索引。
type scenarioReport struct {
	mu     sync.Mutex
	traces map[string]scenarioTrace
}

func newScenarioReport() *scenarioReport {
	return &scenarioReport{traces: make(map[string]scenarioTrace)}
}

func (r *scenarioReport) record(name string, sc trace.SpanContext) {
	if !sc.IsValid() {
		return
	}
	r.mu.Lock()
	r.traces[name] = scenarioTrace{TraceID: sc.TraceID().String(), SpanID: sc.SpanID().String()}
	r.mu.Unlock()
}

func (r *scenarioReport) snapshot() map[string]scenarioTrace {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]scenarioTrace, len(r.traces))
	for name, value := range r.traces {
		out[name] = value
	}
	return out
}

// emitAll 通过真实 Gin 请求生成 HTTP 语义事件，再补充两个无 AccessEvent 的后台错误。
func emitAll(ctx context.Context, logger *log.Logger, tracer trace.Tracer) (*scenarioReport, error) {
	report := newScenarioReport()
	engine := newScenarioEngine(logger, tracer, report)

	requests := []struct {
		method    string
		path      string
		requestID string
		want      int
	}{
		{http.MethodPost, "/api/v1/orders/ORD-1001/pay", requestBusinessSuccess, http.StatusOK},
		{http.MethodPost, "/api/v1/orders", requestBusinessFailed, http.StatusConflict},
		{http.MethodPost, "/api/v1/admin/users/u-2002/role", requestSystemError, http.StatusInternalServerError},
		{http.MethodGet, "/api/v1/panic", requestPanic, http.StatusInternalServerError},
	}
	for _, item := range requests {
		req := httptest.NewRequest(item.method, item.path, nil).WithContext(ctx)
		req.Header.Set("X-Request-ID", item.requestID)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != item.want {
			return nil, fmt.Errorf("%s %s status=%d, want %d", item.method, item.path, rec.Code, item.want)
		}
	}

	emitBackgroundErrors(ctx, logger, tracer, report)
	return report, nil
}

func newScenarioEngine(logger *log.Logger, tracer trace.Tracer, report *scenarioReport) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	requestID := func(c *gin.Context) string { return c.GetHeader("X-Request-ID") }
	engine.Use(
		ginmw.Trace(ginmw.TraceConfig{Tracer: tracer}),
		captureScenarioTrace(report),
		ginmw.AccessLog(ginmw.AccessConfig{Logger: logger}),
		ginmw.Recover(ginmw.RecoverConfig{Logger: logger, GetRequestID: requestID}),
		ginmw.ErrorResponse(ginmw.ErrorConfig{
			Logger:       logger,
			GetRequestID: requestID,
			InputGuard:   blackboxInputGuard,
		}),
	)

	engine.POST("/api/v1/orders/:id/pay", func(c *gin.Context) {
		pauseForLatency()
		logger.Emit(c.Request.Context(), log.BusinessEvent{
			EventMetadata: log.EventMetadata{
				Timestamp: time.Now(),
				Level:     log.LevelInfo,
				RequestID: requestID(c),
			},
			Data: log.BusinessPayload{
				EventName: log.NewEventName("order", "payment", "succeeded"),
				Result:    log.ResultSuccess,
				ExtraAttrs: []slog.Attr{
					slog.String("app.order_id", c.Param("id")),
					slog.Int64("app.amount_cents", 9900),
				},
			},
		})
		c.JSON(http.StatusOK, gin.H{"status": "paid"})
	})

	engine.POST("/api/v1/orders", func(c *gin.Context) {
		pauseForLatency()
		ginmw.Abort(c, mustBusinessError(errs.BusinessErrorConfig{
			Code: "ORDER.CREATE.STOCK_INSUFFICIENT", Type: "business.stock_insufficient", Message: "商品库存不足",
		}))
	})

	engine.POST("/api/v1/admin/users/:id/role", func(c *gin.Context) {
		pauseForLatency()
		ctx := httperr.WithInputSummary(c.Request.Context(), httperr.InputSummary{
			Fields:    []string{"role"},
			Hash:      "sha256:9f2c8d4a",
			Truncated: `{"role":"admin"}`,
		})
		c.Request = c.Request.WithContext(ctx)
		ginmw.Abort(c, mustSystemError(errs.SystemErrorConfig{
			Type: errs.TypeDBQueryTimeout, Message: "update user role: database timeout",
		}))
	})

	engine.GET("/api/v1/panic", func(*gin.Context) {
		pauseForLatency()
		panic("runtime error: invalid memory address")
	})
	return engine
}

func captureScenarioTrace(report *scenarioReport) gin.HandlerFunc {
	return func(c *gin.Context) {
		report.record(c.GetHeader("X-Request-ID"), trace.SpanContextFromContext(c.Request.Context()))
		c.Next()
	}
}

func blackboxInputGuard(ctx context.Context, req *http.Request, _ error, _ httperr.InputSummary) []log.EventPayload {
	if req.Header.Get("X-Request-ID") != requestSystemError {
		return nil
	}
	securityMetadata := httperr.EventMetadataFromContext(ctx)
	securityMetadata.Level = log.LevelWarn
	securityMetadata.RequestID = requestSystemError
	auditMetadata := securityMetadata
	auditMetadata.Level = log.LevelInfo
	return []log.EventPayload{
		log.SecurityEvent{
			EventMetadata: securityMetadata,
			Data: log.SecurityPayload{
				EventName:         log.EventNameInputThreatDetected,
				Subject:           log.Subject{UserID: "u-1001"},
				SecurityEventType: "input.bypass",
				FailureReason:     "非法输入穿透校验触发系统错误",
				ActionTaken:       "blocked",
				RiskScore:         85,
				Result:            log.ResultBlocked,
			},
		},
		log.AuditEvent{
			EventMetadata: auditMetadata,
			Data: log.AuditPayload{
				EventName:      log.EventNameInputAnomalyRecorded,
				Action:         "user.role.update",
				Actor:          log.Actor{UserID: "u-1001", Role: "operator"},
				Resource:       log.Resource{Type: "user", ID: "u-2002"},
				AuditEventType: "input.anomaly",
				TargetUserID:   "u-2002",
				ChangedFields:  []string{"role"},
				Reason:         "高风险输入触发敏感资源操作",
				Result:         log.ResultBlocked,
			},
		},
	}
}

func emitBackgroundErrors(ctx context.Context, logger *log.Logger, tracer trace.Tracer, report *scenarioReport) {
	mqCtx, mqSpan := tracer.Start(ctx, "background mq publish")
	report.record("background-mq", mqSpan.SpanContext())
	logger.Emit(mqCtx, log.EventFromError(
		log.NewEventName("messaging", "publish", "failed"),
		mustSystemError(errs.SystemErrorConfig{
			Type: errs.TypeMQPublishFailed, Message: "publish order.created: deadline exceeded",
			Upstream: "kafka", Retryable: true, Retries: 3, RetriesExhausted: true,
		}),
		log.EventMetadata{},
	))
	mqSpan.End()

	lockCtx, lockSpan := tracer.Start(ctx, "background lock conflict")
	report.record("background-lock", lockSpan.SpanContext())
	logger.Emit(lockCtx, log.EventFromError(
		log.NewEventName("lock", "acquire", "failed"),
		mustSystemError(errs.SystemErrorConfig{
			Type: errs.TypeLockConflict, Message: "acquire lock order:pay:42 conflict",
		}),
		log.EventMetadata{},
	))
	lockSpan.End()
}

func mustBusinessError(cfg errs.BusinessErrorConfig) errs.BizError {
	err, buildErr := errs.NewBusinessError(cfg)
	if buildErr != nil {
		panic(buildErr)
	}
	return err
}

func mustSystemError(cfg errs.SystemErrorConfig) errs.SystemError {
	err, buildErr := errs.NewSystemError(cfg)
	if buildErr != nil {
		panic(buildErr)
	}
	return err
}

func pauseForLatency() {
	time.Sleep(2 * time.Millisecond)
}
