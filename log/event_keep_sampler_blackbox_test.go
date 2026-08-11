// Package log_test 外部黑盒测试：EventKeepSampler 推荐策略在 Logger 管线的端到端效果。
// 期望来自公开事件语义与推荐采样策略，不读取实现内部。
package log_test

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/formal-you/go-observability/log"
)

// recordingWriter 记录通过采样判定并写出的事件类型（msg=event_type）。
type recordingWriter struct {
	msgs []string
}

func (w *recordingWriter) Write(_ context.Context, msg string, _ ...slog.Attr) error {
	w.msgs = append(w.msgs, msg)
	return nil
}

// TestEventKeepSamplerPolicyBlackBox 按推荐策略验证：业务成功全量、access 失败恒保留、
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
			EventName: "business.order.paid",
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

	want := []string{"business", "access"} // msg = event_type；access success 应被丢弃
	if !reflect.DeepEqual(w.msgs, want) {
		t.Errorf("written = %v, want %v", w.msgs, want)
	}
}
