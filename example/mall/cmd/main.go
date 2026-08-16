// Command mall 是端到端参考服务：把 mall 事件注册表、错误体系、治理
// （Sampler / Masker）和 Gin 全链路中间件组合进一个可运行的小商城。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/mall/cmd
//	curl http://127.0.0.1:8083/api/v1/products/42
//	curl -X POST -H 'X-Fail: stock' http://127.0.0.1:8083/api/v1/orders
//
// 默认写 logs/mall.jsonl；设置 OTEL_EXPORTER_OTLP_ENDPOINT 后统一改走 OTLP。
// 本服务不是生产模板，而是展示“同一服务内如何组合库的多个能力”。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/formal-you/go-observability/errs"
	"github.com/formal-you/go-observability/example/mall"
	log "github.com/formal-you/go-observability/log"
	ginmw "github.com/formal-you/go-observability/middleware/gin"
	"github.com/formal-you/go-observability/middleware/httperr"
	"github.com/formal-you/go-observability/middleware/otelutil"
	"github.com/formal-you/go-observability/telemetry"
)

// init 在启动期一次性注册 Error Registry；注册输入是常量，失败即编码错误。
func init() {
	errs.MustRegisterErrorCode("ORDER.CREATE.STOCK_INSUFFICIENT", errs.TypeFailedPrecondition)
	mustRegisterErrorContract("INFRA.MYSQL.QUERY_TIMEOUT", errs.TypeDeadlineExceeded, "user.role_update.database_timeout")
}

// mustRegisterErrorContract 在注册前先校验错误事件名，确保非法事件名在启动期 fail-fast，
// 而不是等到请求期 EventFromError 才暴露。
func mustRegisterErrorContract(code errs.ErrorCode, typ errs.ErrorType, eventName string) {
	if err := log.EventName(eventName).Validate(); err != nil {
		panic("mall: invalid error event name: " + err.Error())
	}
	errs.MustRegisterErrorContract(code, typ, eventName)
}

// fakeIdentity 模拟认证中间件：从请求头读取已认证身份并写入 ctx。
// 真实系统由认证/授权边界实现；这里演示可信 Subject / Actor 注入，
// Logger 经 ContextIdentityExtractor 覆盖事件里的同名字段，防止业务代码伪造身份。
func fakeIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		role := c.GetHeader("X-User-Role")
		if userID == "" {
			userID = "anonymous"
		}
		c.Request = c.Request.WithContext(log.WithIdentityContext(c.Request.Context(), log.IdentityContext{
			Subject: log.Subject{UserID: userID},
			Actor:   log.Actor{UserID: userID, Role: role},
		}))
		c.Next()
	}
}

// mallInputGuard 在错误出口补发审计事件，携带失败输入摘要（app.input_*）。
// 仅当 handler 经 WithInputSummary 挂载了输入摘要时触发，避免为无输入错误的失败制造噪音。
func mallInputGuard(ctx context.Context, r *http.Request, _ error, summary httperr.InputSummary) []log.EventPayload {
	if len(summary.Fields) == 0 {
		return nil
	}
	md := httperr.EventMetadataFromContext(ctx)
	md.Level = log.LevelInfo
	md.RequestID = r.Header.Get("X-Request-ID")
	extra := summary.Attrs()
	// 低敏、低基数参数值可直接落日志；敏感值应使用 Hash / Truncated，不落原文。
	if productID := r.URL.Query().Get("product_id"); productID != "" {
		extra = append(extra, slog.String(string(mall.KeyProductID), productID))
	}
	if quantity := r.URL.Query().Get("quantity"); quantity != "" {
		extra = append(extra, slog.String(string(mall.KeyQuantity), quantity))
	}
	return []log.EventPayload{log.AuditEvent{
		EventMetadata: md,
		Data: log.AuditPayload{
			EventName:      log.EventNameInputAnomalyRecorded,
			Action:         "order.create.rejected",
			AuditEventType: "business.rejection",
			Resource:       log.Resource{Type: "order"},
			Reason:         "业务拒绝，记录失败输入摘要",
			Result:         log.ResultDenied,
			ExtraAttrs:     extra,
		},
	}}
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "mall:", err)
		os.Exit(1)
	}
}

// run 装配三信号、Logger 治理与 Gin 全链路中间件，然后启动 HTTP 服务。
func run(ctx context.Context) error {
	// 默认本地演示：Log 写 file，Trace 走 local，Metric 关闭；
	// 设置 OTEL_EXPORTER_OTLP_ENDPOINT 后三信号统一改走 OTLP。
	logOutput := telemetry.SignalOutputFile
	traceOutput := telemetry.SignalOutputLocal
	metricOutput := telemetry.SignalOutputNone
	filePath := "logs/mall.jsonl"
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		logOutput = telemetry.SignalOutputOTLP
		traceOutput = telemetry.SignalOutputOTLP
		metricOutput = telemetry.SignalOutputOTLP
		filePath = ""
	}

	runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
		Enabled:  telemetry.EnabledFromEnvironment(),
		Endpoint: telemetry.EndpointFromEnvironment(),
		Resource: telemetry.ResourceConfig{ServiceName: "mall", ServiceVersion: "0.1.0", Environment: "dev"},
		Trace:    telemetry.TraceConfig{Output: traceOutput},
		Metric:   telemetry.MetricConfig{Output: metricOutput},
		Log:      telemetry.LogConfig{Output: logOutput, FilePath: filePath},
	})
	if err != nil {
		return err
	}
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	w, err := runtime.NewLogWriter(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = w.Close(ctx) }()

	// 治理策略：
	//   - WithTraceExtractor：事件自动从 request context 关联 trace/span；
	//   - FieldMasker：按键名递归脱敏（使用默认敏感键，业务 PII 可按需追加）；
	//   - EventTypeKeepSampler：business/error/security/audit 恒保留，
	//     access/probe 交给 ResultKeepSampler（示例 ratio=1 便于本地观察）。
	logger := log.NewLogger(w,
		log.WithTraceExtractor(otelutil.NewTraceExtractor()),
		log.WithIdentityExtractor(log.ContextIdentityExtractor{}),
		log.WithMasker(log.FieldMasker{}),
		log.WithSampler(log.EventTypeKeepSampler{
			KeepTypes: []log.EventType{log.EventBusiness, log.EventError, log.EventSecurity, log.EventAudit},
			Fallback:  log.NewResultKeepSampler(1.0),
		}),
	)

	// 业务错误在装配期构造：构造失败返回 error，由 main 统一 os.Exit(1)，
	// 避免请求路径 panic。
	stockInsufficientErr, err := errs.NewBusinessError(errs.BusinessErrorConfig{
		Code:    "ORDER.CREATE.STOCK_INSUFFICIENT",
		Type:    errs.TypeFailedPrecondition,
		Message: "商品库存不足",
	})
	if err != nil {
		return fmt.Errorf("mall: build stock insufficient error: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	requestID := func(c *gin.Context) string { return c.GetHeader("X-Request-ID") }

	// 中间件顺序即执行顺序：Trace -> Access -> Recover -> Metrics -> Security ->
	// Audit -> ErrorResponse。AccessLog 必须包在 Recover 外层，才能在 panic
	// 被收口后记录最终 500 响应；Security/Audit 用回调式 Decide/Describe。
	router.Use(
		ginmw.Trace(ginmw.TraceConfig{}),
		fakeIdentity(),
		ginmw.AccessLog(ginmw.AccessConfig{
			Logger:    logger,
			SkipPaths: map[string]bool{"/healthz": true},
		}),
		ginmw.Recover(ginmw.RecoverConfig{Logger: logger, GetRequestID: requestID}),
		ginmw.Metrics(ginmw.MetricsConfig{}),
		ginmw.SecurityLog(ginmw.SecurityConfig{
			Logger:       logger,
			GetRequestID: requestID,
			Decide: func(c *gin.Context) *log.SecurityPayload {
				if c.GetHeader("X-Risk") != "high" {
					return nil
				}
				return &log.SecurityPayload{
					EventName:         log.NewEventName("auth", "login", "denied"),
					Subject:           log.Subject{UserID: c.GetHeader("X-User-ID")},
					SecurityEventType: "auth.failure",
					FailureReason:     "risk_score_below_threshold",
					ActionTaken:       "blocked",
					RiskScore:         90,
					Result:            log.ResultDenied,
				}
			},
		}),
		ginmw.AuditLog(ginmw.AuditConfig{
			Logger:       logger,
			GetRequestID: requestID,
			Describe: func(c *gin.Context) *log.AuditPayload {
				if c.FullPath() != "/api/v1/admin/users/:id/role" {
					return nil
				}
				return &log.AuditPayload{
					EventName:      log.NewEventName("admin", "role_update", "recorded"),
					Action:         "admin.role_update",
					Resource:       log.Resource{Type: "user", ID: c.Param("id")},
					AuditEventType: "admin.operation",
					TargetUserID:   c.Param("id"),
					ChangedFields:  []string{"role"},
					Before:         log.Fields{"role": "viewer"},
					After:          log.Fields{"role": "operator"},
					Reason:         "授权变更",
					Result:         log.ResultSuccess,
					ExtraAttrs: []slog.Attr{
						slog.String("client.address", c.ClientIP()),
						slog.String("user_agent.original", c.Request.UserAgent()),
					},
				}
			},
		}),
		ginmw.ErrorResponse(ginmw.ErrorConfig{
			Logger:       logger,
			GetRequestID: requestID,
			InputGuard:   mallInputGuard,
			EventNameResolver: func(err error) log.EventName {
				// 框架不提供泛化错误事件名（ADR-0018），接入方必须按实际错误反查注册表，
				// 禁止按 Kind 固定返回同一事件名。
				var appErr errs.AppError
				if !errors.As(err, &appErr) {
					return mall.EventSystemError
				}
				// 系统/基础设施错误：Error Registry 反查 code → type + 错误事件名。
				if name, ok := appErr.ErrCode().RegisteredEventName(); ok {
					return log.EventName(name)
				}
				// 业务/校验错误：从 mall 自建注册表按错误码映射领域事件名。
				if name, ok := mall.EventNameForErrorCode(appErr.ErrCode()); ok {
					return name
				}
				return mall.EventSystemError
			},
		}),
	)

	// 健康检查被 AccessLog.SkipPaths 确定性排除，不产生 access 噪音。
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 商品详情：使用 mall 注册表的事件名与 app.* 扩展键。
	router.GET("/api/v1/products/:id", func(c *gin.Context) {
		productID := c.Param("id")
		logger.Emit(c.Request.Context(), log.BusinessEvent{
			EventMetadata: log.EventMetadata{Level: log.LevelInfo, RequestID: requestID(c)},
			Data: log.BusinessPayload{
				EventName: mall.EventProductViewed,
				Resource:  log.Resource{Type: "product", ID: productID},
				Result:    log.ResultSuccess,
				ExtraAttrs: []slog.Attr{
					slog.String(string(mall.KeyProductID), productID),
				},
			},
		})
		c.JSON(http.StatusOK, gin.H{"id": productID})
	})

	// 下单：默认成功并发出 mall.EventOrderCreated；X-Fail: stock 时走业务拒绝，
	// 由 ginmw.Abort 挂载错误，ErrorResponse 收口渲染 409 并写错误事件。
	router.POST("/api/v1/orders", func(c *gin.Context) {
		orderID := "ORD-1001"
		if c.GetHeader("X-Fail") == "stock" {
			ctx := httperr.WithInputSummary(c.Request.Context(), httperr.InputSummary{
				Fields: []string{"product_id", "quantity"},
			})
			c.Request = c.Request.WithContext(ctx)
			ginmw.Abort(c, stockInsufficientErr)
			return
		}
		logger.Emit(c.Request.Context(), log.BusinessEvent{
			EventMetadata: log.EventMetadata{Level: log.LevelInfo, RequestID: requestID(c)},
			Data: log.BusinessPayload{
				EventName: mall.EventOrderCreated,
				Resource:  log.Resource{Type: "order", ID: orderID},
				Result:    log.ResultSuccess,
				ExtraAttrs: []slog.Attr{
					slog.String(string(mall.KeyOrderID), orderID),
				},
			},
		})
		c.JSON(http.StatusOK, gin.H{"order_id": orderID})
	})

	// 敏感资源变更：由 AuditLog 的 Describe 在链尾写出 AuditEvent。
	router.POST("/api/v1/admin/users/:id/role", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	})

	fmt.Println("mall listening on :8083")
	return router.Run(":8083")
}
