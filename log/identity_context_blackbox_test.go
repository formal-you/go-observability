package log_test

import (
	"context"
	"log/slog"
	"testing"

	obslog "github.com/formal-you/go-observability/log"
)

type identityCaptureWriter struct {
	attrs map[string]slog.Value
}

func (w *identityCaptureWriter) Write(_ context.Context, _ string, attrs ...slog.Attr) error {
	w.attrs = make(map[string]slog.Value, len(attrs))
	for _, attr := range attrs {
		w.attrs[attr.Key] = attr.Value
	}
	return nil
}

func TestIdentityContextBlackBox(t *testing.T) {
	writer := &identityCaptureWriter{}
	logger := obslog.NewLogger(writer,
		obslog.WithIdentityExtractor(obslog.ContextIdentityExtractor{}),
	)
	ctx := obslog.WithIdentityContext(context.Background(), obslog.IdentityContext{
		Subject: obslog.Subject{UserID: "trusted-user", TenantID: "trusted-tenant"},
		Actor:   obslog.Actor{UserID: "trusted-actor", Role: "operator"},
	})

	logger.Emit(ctx, obslog.BusinessEvent{
		EventMetadata: obslog.EventMetadata{Level: obslog.LevelInfo},
		Data: obslog.BusinessPayload{
			EventName: "order.payment.succeeded",
			Subject:   obslog.Subject{UserID: "forged-user", TenantID: "forged-tenant"},
			Result:    obslog.ResultSuccess,
		},
	})
	if got := writer.attrs["user.id"].String(); got != "trusted-user" {
		t.Fatalf("user.id = %q, want trusted-user", got)
	}
	if got := writer.attrs["app.tenant_id"].String(); got != "trusted-tenant" {
		t.Fatalf("app.tenant_id = %q, want trusted-tenant", got)
	}
	if _, exists := writer.attrs["app.user_id"]; exists {
		t.Fatal("new events must not emit deprecated app.user_id")
	}
	if _, exists := writer.attrs["app.actor_user_id"]; exists {
		t.Fatal("BusinessEvent must not receive Actor fields")
	}

	logger.Emit(ctx, obslog.AuditEvent{
		EventMetadata: obslog.EventMetadata{Level: obslog.LevelInfo},
		Data: obslog.AuditPayload{
			EventName: "account.role.changed",
			Actor:     obslog.Actor{UserID: "forged-actor", Role: "forged-role"},
			Result:    obslog.ResultSuccess,
		},
	})
	if got := writer.attrs["app.actor_user_id"].String(); got != "trusted-actor" {
		t.Fatalf("app.actor_user_id = %q, want trusted-actor", got)
	}
	if got := writer.attrs["app.actor_role"].String(); got != "operator" {
		t.Fatalf("app.actor_role = %q, want operator", got)
	}
}

func TestIdentityContextMissingLeavesExplicitSubjectBlackBox(t *testing.T) {
	writer := &identityCaptureWriter{}
	logger := obslog.NewLogger(writer,
		obslog.WithIdentityExtractor(obslog.ContextIdentityExtractor{}),
	)
	logger.Emit(context.Background(), obslog.AccessEvent{
		EventMetadata: obslog.EventMetadata{Level: obslog.LevelInfo},
		Data: obslog.AccessPayload{
			EventName: "http.request.completed",
			Subject:   obslog.Subject{UserID: "explicit-user", TenantID: "explicit-tenant"},
			Result:    obslog.ResultSuccess,
		},
	})
	if got := writer.attrs["user.id"].String(); got != "explicit-user" {
		t.Fatalf("user.id = %q, want explicit-user", got)
	}
	if got := writer.attrs["app.tenant_id"].String(); got != "explicit-tenant" {
		t.Fatalf("app.tenant_id = %q, want explicit-tenant", got)
	}
}
