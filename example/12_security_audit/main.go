// Command security_audit 演示 Gin 的 SecurityLog 与 AuditLog 中间件。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/12_security_audit
//	Get-Content .\logs\security-audit.jsonl
//
// 教学要点：
//   - 推荐回调式 Decide / Describe：认证/授权边界只负责给出判定，库只负责写成事件；
//   - SecurityLog 默认 WARN，AuditLog 默认 INFO；
//   - 本示例用 httptest 触发请求，让程序自身可复现，无需手动 curl。
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/gin-gonic/gin"

	log "github.com/formal-you/go-observability/log"
	ginmw "github.com/formal-you/go-observability/middleware/gin"
	"github.com/formal-you/go-observability/middleware/otelutil"
	"github.com/formal-you/go-observability/telemetry"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "security_audit:", err)
		os.Exit(1)
	}
}

// run 构造 file-only Runtime、Logger 与 Gin 路由，再用 httptest 触发两类事件。
func run(ctx context.Context) error {
	runtime, err := telemetry.NewFileRuntime(telemetry.Config{
		Resource: telemetry.ResourceConfig{ServiceName: "security-audit-demo", ServiceVersion: "0.1.0", Environment: "dev"},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/security-audit.jsonl"},
	})
	if err != nil {
		return err
	}
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	w, err := runtime.NewWriter(ctx)
	if err != nil {
		return err
	}
	defer closeWriter(ctx, w)

	logger := log.NewLogger(w, log.WithTraceExtractor(otelutil.NewTraceExtractor()))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	requestID := func(c *gin.Context) string { return c.GetHeader("X-Request-ID") }

	router.Use(
		// Trace 先创建 server span，SecurityLog / AuditLog 才能把 trace/span
		// 自动关联到同一请求的安全/审计事件上。
		ginmw.Trace(ginmw.TraceConfig{}),
		ginmw.SecurityLog(ginmw.SecurityConfig{
			Logger:       logger,
			GetRequestID: requestID,
			Decide: func(c *gin.Context) *log.SecurityPayload {
				// Decide 返回 nil 表示本次请求不产生 SecurityEvent。
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
				// Describe 只在命中敏感路由时返回审计载荷，避免普通请求刷审计事件。
				if c.FullPath() != "/api/v1/admin/users/:id/role" {
					return nil
				}
				return &log.AuditPayload{
					EventName:      log.NewEventName("admin", "role_update", "recorded"),
					Action:         "admin.role_update",
					Actor:          log.Actor{UserID: "u_admin", Role: "admin"},
					Resource:       log.Resource{Type: "user", ID: c.Param("id")},
					AuditEventType: "admin.operation",
					TargetUserID:   c.Param("id"),
					ChangedFields:  []string{"role"},
					After:          log.Fields{"role": "operator"},
					Reason:         "授权变更",
					Result:         log.ResultSuccess,
				}
			},
		}),
	)

	router.POST("/api/v1/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/api/v1/admin/users/:id/role", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	})

	// fire 用 httptest 直接经过真实中间件链，避免启动长驻 Server。
	fire := func(method, path string, headers map[string]string) int {
		req := httptest.NewRequest(method, path, nil)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	fmt.Printf("login status=%d\n", fire(http.MethodPost, "/api/v1/login", map[string]string{
		"X-Request-ID": "req-001",
		"X-User-ID":    "u_1001",
		"X-Risk":       "high",
	}))
	fmt.Printf("admin role status=%d\n", fire(http.MethodPost, "/api/v1/admin/users/u-2002/role", map[string]string{
		"X-Request-ID": "req-002",
		"X-User-ID":    "u_admin",
	}))

	fmt.Println("written: logs/security-audit.jsonl")
	return nil
}

// closeWriter 在函数返回前释放 Writer 自己拥有的资源。
func closeWriter(ctx context.Context, w log.ManagedWriter) {
	_ = w.Close(ctx)
}
