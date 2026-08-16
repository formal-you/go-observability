// Command sampler_masker 演示 Logger 治理管线的两个显式注入点：Sampler 与 Masker。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/04_sampler_masker
//	Get-Content .\logs\governance.jsonl
//
// 预期输出：只有 error / security / audit 三条事件；error 中的 app.secret
// 已被替换为 ***。治理顺序固定为 metadata 补全 -> 最低级别过滤 -> Masker ->
// Sampler -> Write，因此脱敏先于采样，采样后事件才真正交给 Writer。
package main

import (
	"context"
	"fmt"
	"os"

	"log/slog"

	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/logwriter/file"
)

func main() {
	if err := run(context.Background(), "logs/governance.jsonl"); err != nil {
		fmt.Fprintln(os.Stderr, "sampler_masker:", err)
		os.Exit(1)
	}
}

// run 是示例的可测试主体：把输出路径作为参数，避免把教学逻辑困死在 main 里。
func run(ctx context.Context, path string) error {
	// 先删除旧文件，保证多次运行得到同一份可对照输出。
	_ = os.Remove(path)

	w, err := file.New(path)
	if err != nil {
		return err
	}

	// FieldMasker 按键名（精确或后缀）递归脱敏；这里追加 app.secret，
	// 与 DefaultSensitiveKeys 提供的凭证/密钥清单叠加。
	//
	// EventTypeKeepSampler 是推荐的粗分类保留策略：error/security/audit 恒保留，
	// 其余事件交给 Fallback。Fallback 显式返回 false，让本示例输出确定，便于课堂核对；
	// 生产环境通常把 access 交给 ResultKeepSampler 做成功噪音采样。
	logger := log.NewLogger(w,
		log.WithMasker(log.FieldMasker{Keys: []string{"app.secret"}}),
		log.WithSampler(log.EventTypeKeepSampler{
			KeepTypes: []log.EventType{log.EventError, log.EventSecurity, log.EventAudit},
			Fallback:  log.SamplerFunc(func(context.Context, []slog.Attr) bool { return false }),
		}),
	)

	emit := func(event log.EventPayload) {
		logger.Emit(ctx, event)
	}

	// access 与 business 不属于 KeepTypes，会被 Fallback 丢弃。
	emit(log.AccessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.AccessPayload{
			EventName: log.EventNameHTTPRequestCompleted,
			HTTP:      log.HTTPInfo{Method: "GET", URLPath: "/healthz", StatusCode: 200},
			Result:    log.ResultSuccess,
		},
	})
	emit(log.BusinessEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.BusinessPayload{
			EventName: log.NewEventName("order", "payment", "succeeded"),
			Result:    log.ResultSuccess,
		},
	})

	// error 被 EventTypeKeepSampler 恒保留；ExtraAttrs 中的 app.secret
	// 会先经 FieldMasker 替换为 ***，再进入采样判定。
	emit(log.ErrorEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelError},
		Data: log.ErrorPayload{
			EventName:    log.NewEventName("order", "payment", "gateway_timeout"),
			ErrorType:    "DEADLINE_EXCEEDED",
			ErrorMessage: "payment gateway timeout",
			Result:       log.ResultError,
			ExtraAttrs:   []slog.Attr{slog.String("app.secret", "should-be-redacted")},
		},
	})

	// security / audit 是高价值事件，必须保留；本例让读者直接看到它们的类型列。
	emit(log.SecurityEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelWarn},
		Data: log.SecurityPayload{
			EventName: log.NewEventName("auth", "login", "denied"),
			Result:    log.ResultDenied,
		},
	})
	emit(log.AuditEvent{
		EventMetadata: log.EventMetadata{Level: log.LevelInfo},
		Data: log.AuditPayload{
			EventName: log.NewEventName("admin", "role_update", "recorded"),
			Result:    log.ResultSuccess,
		},
	})

	if err := w.Close(ctx); err != nil {
		return err
	}
	fmt.Println("written: logs/governance.jsonl (expected: 3 events; app.secret is redacted)")
	return nil
}
