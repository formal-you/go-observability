package ginmw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/formal-you/go-observability/log"
)

// TestSecurityLogEmitsSetSecurityEvent 验证 SecurityLog：handler 经 SetSecurity 挂载
// 载荷后，链尾写出 SecurityEvent（缺省 WARN、trace/span/request_id 关联）。
func TestSecurityLogEmitsSetSecurityEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(SecurityLog(SecurityConfig{
		Logger:       logger,
		GetRequestID: func(*gin.Context) string { return "req-sec-1" },
	}))
	engine.POST("/login", func(c *gin.Context) {
		SetSecurity(c, log.SecurityPayload{
			EventName:         log.EventNameSecurityInputAnomaly,
			Subject:           log.Subject{UserID: "u-1001"},
			SecurityEventType: "auth.bypass",
			FailureReason:     "非法输入穿透认证",
			ActionTaken:       "blocked",
			RiskScore:         90,
			Result:            log.ResultBlocked,
		})
		c.String(http.StatusOK, "ok")
	})

	tid, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	sid, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled})
	req := httptest.NewRequest(http.MethodPost, "/login", nil).WithContext(trace.ContextWithRemoteSpanContext(context.Background(), sc))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if len(w.msgs) != 1 || w.msgs[0] != "security" {
		t.Fatalf("msgs = %v, want [security]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "security.input.anomaly")
	attrString(t, attrs, "app.security_event_type", "auth.bypass")
	attrString(t, attrs, "app.action_taken", "blocked")
	attrString(t, attrs, "app.risk_score", "90")
	attrString(t, attrs, "app.result", "blocked")
	attrString(t, attrs, "level", "WARN")
	attrString(t, attrs, "request_id", "req-sec-1")
	attrString(t, attrs, "trace_id", "0123456789abcdef0123456789abcdef")
	attrString(t, attrs, "span_id", "0123456789abcdef")
}

// TestSecurityLogNoPayloadNoEvent 验证未调用 SetSecurity 时 SecurityLog 直接放行、不写事件。
func TestSecurityLogNoPayloadNoEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(SecurityLog(SecurityConfig{Logger: logger}))
	engine.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := doRequest(engine, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(w.msgs) != 0 {
		t.Errorf("未挂载安全载荷不应写事件，实际 %v", w.msgs)
	}
}

// TestSecurityLogCustomLevel 验证 SecurityConfig.Level 可覆盖缺省 WARN。
func TestSecurityLogCustomLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(SecurityLog(SecurityConfig{Logger: logger, Level: log.LevelError}))
	engine.GET("/x", func(c *gin.Context) {
		SetSecurity(c, log.SecurityPayload{EventName: log.EventNameSecurityInputAnomaly, Result: log.ResultBlocked})
	})

	doRequest(engine, "/x")
	attrString(t, attrMap(w.attrsList[0]), "level", "ERROR")
}

// TestAuditLogEmitsSetAuditEvent 验证 AuditLog：handler 经 SetAudit 挂载载荷后，
// 链尾写出 AuditEvent（缺省 INFO、actor/资源字段完整）。
func TestAuditLogEmitsSetAuditEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(AuditLog(AuditConfig{Logger: logger}))
	engine.POST("/users/:id/role", func(c *gin.Context) {
		SetAudit(c, log.AuditPayload{
			EventName:      log.EventNameAuditInputAnomaly,
			Action:         "user.role.update",
			Actor:          log.Actor{UserID: "u-1001", Role: "operator"},
			Resource:       log.Resource{Type: "user", ID: "u-2002"},
			AuditEventType: "role.change",
			TargetUserID:   "u-2002",
			ChangedFields:  []string{"role"},
			Reason:         "权限变更",
			Result:         log.ResultSuccess,
		})
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/users/u-2002/role", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(w.msgs) != 1 || w.msgs[0] != "audit" {
		t.Fatalf("msgs = %v, want [audit]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "audit.input.anomaly")
	attrString(t, attrs, "app.action", "user.role.update")
	attrString(t, attrs, "app.actor_user_id", "u-1001")
	attrString(t, attrs, "app.actor_role", "operator")
	attrString(t, attrs, "app.target_user_id", "u-2002")
	attrString(t, attrs, "app.result", "success")
	attrString(t, attrs, "level", "INFO")
}

// TestAuditLogNoPayloadNoEvent 验证未调用 SetAudit 时 AuditLog 直接放行、不写事件。
func TestAuditLogNoPayloadNoEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &captureWriter{}
	logger := log.NewLogger(w)
	engine := gin.New()
	engine.Use(AuditLog(AuditConfig{Logger: logger}))
	engine.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := doRequest(engine, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(w.msgs) != 0 {
		t.Errorf("未挂载审计载荷不应写事件，实际 %v", w.msgs)
	}
}
