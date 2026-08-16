// Command errors 演示 errs 错误建模与 log.EventFromError 投影。
//
// 教学要点：
//   - ErrorKind 区分 validation / business / system 三类预期性；
//   - ErrorType 是 OTel/gRPC 标准枚举（低基数、闭合枚举）；
//   - ErrorCode 是 <SCOPE>.<OPERATION>.<REASON> 的具体业务码；
//   - Error Registry 固定 code -> type 映射，漂移必须尽早暴露；
//   - 同一个 error 经 EventFromError 按 Kind 分派成 business 或 error 事件。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/03_errors
//	Get-Content .\logs\errors.jsonl
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/formal-you/go-observability/errs"
	log "github.com/formal-you/go-observability/log"
	"github.com/formal-you/go-observability/telemetry"
)

// init 在进程启动期一次性注册 Error Registry。
// ErrorCode -> ErrorType 的固定映射是查询/告警依据，漂移必须尽早暴露。
func init() {
	errs.MustRegisterErrorCode("ORDER.CREATE.STOCK_INSUFFICIENT", errs.TypeFailedPrecondition)
	errs.MustRegisterErrorContract("INFRA.MYSQL.QUERY_TIMEOUT", errs.TypeDeadlineExceeded, "user.role_update.database_timeout")
	errs.MustRegisterErrorCode("PAY.GATEWAY.TIMEOUT", errs.TypeUnavailable)
}

func main() {
	ctx := context.Background()

	// file-only Runtime 让本示例不依赖 Collector 也能看到错误事件投影。
	runtime, err := telemetry.NewFileRuntime(telemetry.Config{
		Resource: telemetry.ResourceConfig{ServiceName: "errors-demo", ServiceVersion: "0.1.0", Environment: "dev"},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/errors.jsonl"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup telemetry:", err)
		os.Exit(1)
	}
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	w, err := runtime.NewLogWriter(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create writer:", err)
		os.Exit(1)
	}
	defer closeWriter(ctx, w)

	logger := log.NewLogger(w)

	// KindBusiness：预期内业务拒绝，收口层映射为 409，不按系统故障告警。
	biz, err := errs.NewBusinessError(errs.BusinessErrorConfig{
		Code:    "ORDER.CREATE.STOCK_INSUFFICIENT",
		Type:    errs.TypeFailedPrecondition,
		Message: "商品库存不足",
	})
	must(err, "business error")

	// KindValidation：参数/入参校验失败，收口层映射为 400，同属预期内拒绝。
	validation, err := errs.NewValidationError(errs.ValidationErrorConfig{Message: "order_id 必填"})
	must(err, "validation error")

	// KindSystem：非预期故障；这里标记 Retryable=true，LevelOf 会推导为 WARN，
	// 若 RetriesExhausted=true 或不可重试，则推导为 ERROR。
	system, err := errs.NewSystemError(errs.SystemErrorConfig{
		Type:      errs.TypeDeadlineExceeded,
		Code:      "INFRA.MYSQL.QUERY_TIMEOUT",
		Message:   "update user role: database timeout",
		Upstream:  "mysql",
		Retryable: true,
		Retries:   2,
	})
	must(err, "system error")

	// 重试耗尽是不可重试/耗尽路径的代表：LevelOf 应为 ERROR。
	retryExhausted, err := errs.NewSystemError(errs.SystemErrorConfig{
		Type:             errs.TypeUnavailable,
		Code:             "PAY.GATEWAY.TIMEOUT",
		Message:          "payment gateway timeout",
		Upstream:         "payment-gateway",
		Retryable:        true,
		Retries:          3,
		RetriesExhausted: true,
	})
	must(err, "retry exhausted error")

	// 先看内存中的分类结果，再写文件；两类输出互为对照。
	fmt.Printf("biz      level=%s kind=%s code=%s type=%s\n", log.LevelOf(biz), biz.Kind(), biz.ErrCode(), biz.ErrorType())
	fmt.Printf("valid    level=%s kind=%s type=%s\n", log.LevelOf(validation), validation.Kind(), validation.ErrorType())
	fmt.Printf("system   level=%s kind=%s code=%s type=%s retry_exhausted=%t\n", log.LevelOf(system), system.Kind(), system.ErrCode(), system.ErrorType(), errs.IsRetryExhausted(system))
	fmt.Printf("exhaust  level=%s retry_exhausted=%t\n", log.LevelOf(retryExhausted), errs.IsRetryExhausted(retryExhausted))

	// EventFromError 沿错误链提取 errs.AppError，并按 Kind 分派：
	//   - validation / business -> BusinessEvent；
	//   - system / 普通 error -> ErrorEvent。
	// 事件名必须由接入方从注册表传入，框架不自动派生泛化错误事件名。
	logger.Emit(ctx, log.EventFromError(log.NewEventName("order", "create", "stock_insufficient"), biz, log.EventMetadata{}))
	logger.Emit(ctx, log.EventFromError(log.NewEventName("order", "create", "invalid_argument"), validation, log.EventMetadata{}))
	logger.Emit(ctx, log.EventFromError(log.NewEventName("user", "role_update", "database_timeout"), system, log.EventMetadata{}))
	logger.Emit(ctx, log.EventFromError(log.NewEventName("order", "payment", "gateway_timeout"), retryExhausted, log.EventMetadata{}))

	fmt.Println("written: logs/errors.jsonl")
}

// must 把示例里的配置错误尽早转成 panic；生产代码通常显式返回错误。
func must(err error, what string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %v", what, err))
	}
}

// closeWriter 在函数返回前释放 Writer 自己拥有的资源。
func closeWriter(ctx context.Context, w log.ManagedWriter) {
	_ = w.Close(ctx)
}
