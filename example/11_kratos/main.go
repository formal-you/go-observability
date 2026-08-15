// Command kratos 演示 go-kratos v3 传输层适配：HTTP ErrorEncoder、错误日志
// 中间件与 gRPC ErrorMapper。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/11_kratos
//	Get-Content .\logs\kratos.jsonl
//
// 教学要点：
//   - ErrorEncoder 把 errs.AppError 映射为 kratos {code,reason,message,metadata}；
//   - ErrorLog 只负责把 handler 返回的错误写成 ErrorEvent，不改变响应；
//   - GRPCErrorMapper 把错误转成 gRPC status，避免内部 message 以 Unknown 透传；
//   - kratos 原生错误保持原契约，本库只对 errs.AppError / 普通错误做统一收口。
package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"

	"google.golang.org/grpc/status"

	"github.com/formal-you/go-observability/errs"
	log "github.com/formal-you/go-observability/log"
	kratosmw "github.com/formal-you/go-observability/middleware/kratos"
	"github.com/formal-you/go-observability/middleware/otelutil"
	"github.com/formal-you/go-observability/telemetry"
)

// init 在启动期注册错误码，供严格构造器校验 code -> type 映射。
func init() {
	errs.MustRegisterErrorCode("ORDER.CREATE.STOCK_INSUFFICIENT", errs.TypeFailedPrecondition)
	errs.MustRegisterErrorCode("INFRA.MYSQL.QUERY_TIMEOUT", errs.TypeDeadlineExceeded)
}

func main() {
	ctx := context.Background()

	// file-only Runtime 让示例不依赖 Collector，即可看到 ErrorLog 写出的 JSONL。
	runtime, err := telemetry.NewFileRuntime(telemetry.Config{
		Resource: telemetry.ResourceConfig{ServiceName: "kratos-demo", ServiceVersion: "0.1.0", Environment: "dev"},
		Log:      telemetry.LogConfig{Output: telemetry.SignalOutputFile, FilePath: "logs/kratos.jsonl"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup telemetry:", err)
		os.Exit(1)
	}
	restore := runtime.InstallGlobal()
	defer restore()
	defer func() { _ = runtime.Shutdown(ctx) }()

	w, err := runtime.NewWriter(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create writer:", err)
		os.Exit(1)
	}
	defer closeWriter(ctx, w)

	logger := log.NewLogger(w, log.WithTraceExtractor(otelutil.NewTraceExtractor()))

	biz := mustBusinessError(errs.BusinessErrorConfig{
		Code:    "ORDER.CREATE.STOCK_INSUFFICIENT",
		Type:    errs.TypeFailedPrecondition,
		Message: "商品库存不足",
	})
	system := mustSystemError(errs.SystemErrorConfig{
		Type:     errs.TypeDeadlineExceeded,
		Code:     "INFRA.MYSQL.QUERY_TIMEOUT",
		Message:  "update user role: database timeout",
		Upstream: "mysql",
	})

	// 1) HTTP ErrorEncoder：errs.AppError -> kratos 错误契约。
	// 业务拒绝会透传安全 message；系统错误只给固定文案，不泄露内部细节。
	encoder := kratosmw.ErrorEncoder()
	encode := func(name string, err error) {
		rec := httptest.NewRecorder()
		encoder(rec, httptest.NewRequest("GET", "/", nil), err)
		fmt.Printf("%s status=%d body=%s\n", name, rec.Code, rec.Body.String())
	}
	encode("biz", biz)
	encode("system", system)

	// 2) ErrorLog：handler 返回错误时写出 ErrorEvent，再把原错误交还传输层。
	// 真实 kratos 服务应把 ErrorLog 放在 GRPCErrorMapper 外层，记录转换前的原始错误。
	errorLog := kratosmw.ErrorLog(logger, kratosmw.WithGetRequestID(func(context.Context) string { return "req-001" }))
	_, err = errorLog(func(ctx context.Context, req any) (any, error) {
		return nil, biz
	})(ctx, nil)
	fmt.Printf("errorlog returned err=%v\n", err)

	// 3) GRPCErrorMapper：把 errs.AppError 转成 gRPC status。
	// code 来自 errs.Kind 映射，reason 与 error.type 写入 errdetails.ErrorInfo。
	grpcMapper := kratosmw.GRPCErrorMapper()
	_, mappedErr := grpcMapper(func(ctx context.Context, req any) (any, error) {
		return nil, biz
	})(ctx, nil)
	st := status.Convert(mappedErr)
	fmt.Printf("grpc mapped code=%s message=%s\n", st.Code(), st.Message())

	fmt.Println("written: logs/kratos.jsonl")
}

// mustBusinessError 把严格构造错误直接 panic：示例中的配置错误应尽早暴露。
func mustBusinessError(cfg errs.BusinessErrorConfig) errs.BizError {
	err, buildErr := errs.NewBusinessError(cfg)
	if buildErr != nil {
		panic(buildErr)
	}
	return err
}

// mustSystemError 把严格构造错误直接 panic：示例中的配置错误应尽早暴露。
func mustSystemError(cfg errs.SystemErrorConfig) errs.SystemError {
	err, buildErr := errs.NewSystemError(cfg)
	if buildErr != nil {
		panic(buildErr)
	}
	return err
}

// closeWriter 在函数返回前释放 Writer 自己拥有的资源。
func closeWriter(ctx context.Context, w log.ManagedWriter) {
	_ = w.Close(ctx)
}
