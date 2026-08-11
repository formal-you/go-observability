package httpmw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/formal-you/go-observability/log"
)

// TestSecurityLogEmitsSetSecurityEvent 验证 net/http 版 SecurityLog：handler 经
// SetSecurity 挂载载荷后，链尾写出 SecurityEvent（缺省 WARN）。
func TestSecurityLogEmitsSetSecurityEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := SecurityLog(SecurityConfig{
		Logger:       logger,
		GetRequestID: func(*http.Request) string { return "req-sec-1" },
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetSecurity(w, log.SecurityPayload{
			EventName:         log.EventNameSecurityInputAnomaly,
			SecurityEventType: "auth.bypass",
			FailureReason:     "非法输入穿透认证",
			ActionTaken:       "blocked",
			RiskScore:         90,
			Result:            log.ResultBlocked,
		})
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/login", nil))

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
}

// TestSecurityLogNoPayloadNoEvent 验证未调用 SetSecurity 时 SecurityLog 直接放行、不写事件。
func TestSecurityLogNoPayloadNoEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := SecurityLog(SecurityConfig{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if len(w.msgs) != 0 {
		t.Errorf("未挂载安全载荷不应写事件，实际 %v", w.msgs)
	}
}

// TestSecurityLogCustomLevel 验证 SecurityConfig.Level 可覆盖缺省 WARN。
func TestSecurityLogCustomLevel(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := SecurityLog(SecurityConfig{Logger: logger, Level: log.LevelError})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetSecurity(w, log.SecurityPayload{EventName: log.EventNameSecurityInputAnomaly, Result: log.ResultBlocked})
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	attrString(t, attrMap(w.attrsList[0]), "level", "ERROR")
}

// TestAuditLogEmitsSetAuditEvent 验证 net/http 版 AuditLog：handler 经 SetAudit 挂载
// 载荷后，链尾写出 AuditEvent（缺省 INFO、actor/资源字段完整）。
func TestAuditLogEmitsSetAuditEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := AuditLog(AuditConfig{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetAudit(w, log.AuditPayload{
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
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/u-2002/role", nil))

	if len(w.msgs) != 1 || w.msgs[0] != "audit" {
		t.Fatalf("msgs = %v, want [audit]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "audit.input.anomaly")
	attrString(t, attrs, "app.action", "user.role.update")
	attrString(t, attrs, "app.actor_user_id", "u-1001")
	attrString(t, attrs, "app.target_user_id", "u-2002")
	attrString(t, attrs, "app.result", "success")
	attrString(t, attrs, "level", "INFO")
}

// TestAuditLogNoPayloadNoEvent 验证未调用 SetAudit 时 AuditLog 直接放行、不写事件。
func TestAuditLogNoPayloadNoEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := AuditLog(AuditConfig{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if len(w.msgs) != 0 {
		t.Errorf("未挂载审计载荷不应写事件，实际 %v", w.msgs)
	}
}

// TestSecurityLogDecideEmits 验证 net/http 拉取式用法：认证/授权逻辑放进 Decide，
// SecurityLog 链尾自动调用并写出 SecurityEvent（无需手动 SetSecurity）。
func TestSecurityLogDecideEmits(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := SecurityLog(SecurityConfig{
		Logger: logger,
		Decide: func(r *http.Request) *log.SecurityPayload {
			if r.Header.Get("X-Auth") != "valid" {
				return &log.SecurityPayload{
					EventName:         log.EventNameSecurityInputAnomaly,
					SecurityEventType: "auth.denied",
					FailureReason:     "未认证",
					ActionTaken:       "blocked",
					RiskScore:         80,
					Result:            log.ResultBlocked,
				}
			}
			return nil
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200（SecurityLog 只记录不拦截）", rec.Code)
	}
	if len(w.msgs) != 1 || w.msgs[0] != "security" {
		t.Fatalf("msgs = %v, want [security]", w.msgs)
	}
	attrs := attrMap(w.attrsList[0])
	attrString(t, attrs, "event.name", "security.input.anomaly")
	attrString(t, attrs, "app.security_event_type", "auth.denied")
	attrString(t, attrs, "app.risk_score", "80")
	attrString(t, attrs, "app.result", "blocked")
	attrString(t, attrs, "level", "WARN")
}

// TestSecurityLogDecideNilNoEvent 验证 Decide 返回 nil 时不写事件。
func TestSecurityLogDecideNilNoEvent(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := SecurityLog(SecurityConfig{
		Logger: logger,
		Decide: func(r *http.Request) *log.SecurityPayload { return nil },
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if len(w.msgs) != 0 {
		t.Errorf("Decide 返回 nil 不应写事件，实际 %v", w.msgs)
	}
}

// TestAuditLogDescribeEmits 验证 net/http 拉取式用法：Describe 在链尾组装并写出 AuditEvent。
func TestAuditLogDescribeEmits(t *testing.T) {
	w := &captureWriter{}
	logger := log.NewLogger(w)
	handler := AuditLog(AuditConfig{
		Logger: logger,
		Describe: func(r *http.Request) *log.AuditPayload {
			if r.URL.Path != "/users/u-2002/role" {
				return nil
			}
			return &log.AuditPayload{
				EventName:      log.EventNameAuditInputAnomaly,
				Action:         "user.role.update",
				Actor:          log.Actor{UserID: "u-1001", Role: "operator"},
				Resource:       log.Resource{Type: "user", ID: "u-2002"},
				AuditEventType: "role.change",
				TargetUserID:   "u-2002",
				ChangedFields:  []string{"role"},
				Reason:         "权限变更",
				Result:         log.ResultSuccess,
			}
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users/u-2002/role", nil))
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
	attrString(t, attrs, "app.result", "success")
	attrString(t, attrs, "level", "INFO")
}
