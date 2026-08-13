package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	// 优雅地处理中断信号（Ctrl+C）。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 设置并初始化 OpenTelemetry SDK。
	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return err
	}
	// 确保在程序结束之前调用 shutdown 方法清理资源。
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// 启动 HTTP 服务器。
	srv := &http.Server{
		Addr:         ":8080",
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(),
	}
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	// 等待中断信号
	select {
	case err = <-srvErr:
		// 启动 HTTP 服务器时出错。
		return err
	case <-ctx.Done():
		// 等待第一个 CTRL+C 信号。
		// 尽快停止接受信号通知。
		stop()
	}

	// 当调用 Shutdown 时，ListenAndServe 会立即返回 ErrServerClosed 错误。
	err = srv.Shutdown(context.Background())
	return err
}

func newHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// 注册 Handler。
	mux.HandleFunc("/rolldice/", rolldice)
	mux.HandleFunc("/rolldice/{player}", rolldice)

	// 为整个服务器添加 HTTP 插桩处理器。
	handler := otelhttp.NewHandler(mux, "/")
	return handler
}
