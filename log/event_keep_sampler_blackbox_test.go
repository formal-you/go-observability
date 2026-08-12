// Package log_test 外部黑盒测试：EventKeepSampler 显式高流量策略在 Logger 管线的端到端效果。
// 期望来自公开事件语义与可选采样契约，不读取实现内部。
package log_test

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/formal-you/go-observability/log"
)

// recordingWriter 记录通过采样判定并写出的事件类型（type=event_type）。
type recordingWriter struct {
	eventTypes []string
}

func (w *recordingWriter) Write(_ context.Context, eventType string, _ ...slog.Attr) error {
	w.eventTypes = append(w.eventTypes, eventType)
	return nil
}

// TestEventKeepSamplerPolicyBlackBox 按显式高流量策略验证：业务成功全量、access 失败恒保留、
// access 成功被采样丢弃（Fallback Ratio=0 保证确定性）。
func TestEventKeepSamplerPolicyBlackBox(t *testing.T) {
	w := &recordingWriter{}
	logger := log.NewLogger(w,
		log.WithSampler(log.EventKeepSampler{
			KeepPrefixes: []string{"business.", "error.", "security.", "audit.", "probe."},
			Fallback:     log.ResultKeepSampler{Ratio: 0},
		}),
	)
	ctx := context.Background()

	logger.Emit(ctx, log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: "order.payment.succeeded",
			Result:    log.ResultSuccess,
		},
	})
	logger.Emit(ctx, log.AccessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelWarn},
		Data: log.AccessPayload{
			EventName: log.EventNameAccessHTTPRequest,
			HTTP:      log.HTTPInfo{StatusCode: 500},
			Result:    log.ResultFailed,
		},
	})
	logger.Emit(ctx, log.AccessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.AccessPayload{
			EventName: log.EventNameAccessHTTPRequest,
			HTTP:      log.HTTPInfo{StatusCode: 200},
			Result:    log.ResultSuccess,
		},
	})

	want := []string{"business", "access"} // type = event_type；access success 应被丢弃
	if !reflect.DeepEqual(w.eventTypes, want) {
		t.Errorf("written = %v, want %v", w.eventTypes, want)
	}
}
