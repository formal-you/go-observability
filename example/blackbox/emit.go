package main

import (
	"context"
	"errors"
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

// init 注册黑盒用到的 error.code → error.type 映射（Error Registry，启动期一次性写入）。
// SCOPE 归属由失败面决定（ADR-0019）：业务拒绝用业务模块（ORDER）；系统/基础设施故障用
// INFRA（OPERATION=组件：MYSQL/REDIS/MQ）；非基础设施系统类（如并发冲突 LOCK）保留领域 SCOPE。
func init() {
	mustRegisterErrorCode("ORDER.CREATE.STOCK_INSUFFICIENT", errs.TypeFailedPrecondition)
	mustRegisterErrorCode("INFRA.MYSQL.QUERY_TIMEOUT", errs.TypeDeadlineExceeded)
	mustRegisterErrorCode("INFRA.MQ.PUBLISH_TIMEOUT", errs.TypeDeadlineExceeded)
	mustRegisterErrorCode("LOCK.ACQUIRE.CONFLICT", errs.TypeAborted)
	mustRegisterErrorCode("INFRA.REDIS.UNAVAILABLE", errs.TypeUnavailable)
}

func mustRegisterErrorCode(code errs.ErrorCode, typ errs.ErrorType) {
	if err := errs.RegisterErrorCode(code, typ); err != nil {
		panic("blackbox: register error code: " + err.Error())
	}
}

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
			// ADR-0018：框架不提供泛化错误事件名，接入方经 resolver 提供具体事实名。
			EventNameResolver: func(err error) log.EventName {
				var appErr errs.AppError
				if errors.As(err, &appErr) {
					switch appErr.Kind() {
					case errs.KindValidation, errs.KindBusiness:
						return log.NewEventName("order", "create", "stock_insufficient")
					}
				}
				return log.NewEventName("user", "role_update", "database_timeout")
			},
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
			Code: "ORDER.CREATE.STOCK_INSUFFICIENT", Type: "FAILED_PRECONDITION", Message: "商品库存不足",
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
			Type: errs.TypeDeadlineExceeded, Code: "INFRA.MYSQL.QUERY_TIMEOUT", Message: "update user role: database timeout",
			Upstream: "mysql",
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
		log.NewEventName("messaging", "publish", "deadline_exceeded"),
		mustSystemError(errs.SystemErrorConfig{
			Type: errs.TypeDeadlineExceeded, Code: "INFRA.MQ.PUBLISH_TIMEOUT", Message: "publish order.created: deadline exceeded",
			Upstream: "kafka", Retryable: true, Retries: 3, RetriesExhausted: true,
		}),
		log.EventMetadata{},
	))
	mqSpan.End()

	cacheCtx, cacheSpan := tracer.Start(ctx, "background cache unavailable")
	report.record("background-cache", cacheSpan.SpanContext())
	logger.Emit(cacheCtx, log.EventFromError(
		log.NewEventName("cache", "read", "unavailable"),
		mustSystemError(errs.SystemErrorConfig{
			Type: errs.TypeUnavailable, Code: "INFRA.REDIS.UNAVAILABLE", Message: "read cache order:pay:42: redis unavailable",
			Upstream: "redis", Retryable: true, Retries: 2,
		}),
		log.EventMetadata{},
	))
	cacheSpan.End()

	lockCtx, lockSpan := tracer.Start(ctx, "background lock conflict")
	report.record("background-lock", lockSpan.SpanContext())
	logger.Emit(lockCtx, log.EventFromError(
		log.NewEventName("lock", "acquire", "conflict"),
		mustSystemError(errs.SystemErrorConfig{
			Type: errs.TypeAborted, Code: "LOCK.ACQUIRE.CONFLICT", Message: "acquire lock order:pay:42 conflict",
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
