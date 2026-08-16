// Command layered_span 演示 otelutil.WithSpan / StartSpan 如何把一次请求拆成
// handler→service→store→db 的分层调用链（ADR-0023）：Gin 入口的 server span 由
// ginmw.Trace 创建，业务层用 WithSpan 包一层函数并逐层下传 ctx，形成父子 span 树。
//
// 运行方式（从仓库根目录）：
//
//	go run ./example/17_layered_span
//
// 预期：标准输出以 pretty 格式打印 5 个 span（server / handler / service / store / db），
// 父子链 server→handler→service→store→db，共享同一 trace_id。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	ginmw "github.com/formal-you/go-observability/middleware/gin"
	"github.com/formal-you/go-observability/middleware/otelutil"
)

func main() {
	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		slog.Error("init stdout trace exporter", "err", err)
		os.Exit(1)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// 显式安装全局 TracerProvider：ginmw.Trace 与 otelutil 默认都走全局
	// otel.Tracer("go-observability")，示例保持"零注入"的接入方式。
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	engine := newEngine()
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil))
	fmt.Printf("POST /api/v1/orders → %d\n", rec.Code)
	if err := tp.ForceFlush(context.Background()); err != nil {
		slog.Error("flush trace", "err", err)
	}
}

// newEngine 组装 Gin 中间件与分层 handler：server span 由 ginmw.Trace 创建，
// handler 内手动拆 handler→service→store→db 子 span。
func newEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginmw.Trace(ginmw.TraceConfig{}))
	r.POST("/api/v1/orders", layeredHandler())
	return r
}

// layeredHandler 组装入口：handler 层调用 service 层，每层一个函数。
func layeredHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if err := handlerCreateOrder(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// handlerCreateOrder 是 handler 层：本层处理代码（校验、组装）写在调用 service 层之前。
func handlerCreateOrder(ctx context.Context) error {
	_, err := otelutil.WithSpan(ctx, "handler.order.Create", func(ctx context.Context) error {
		// handler 层处理：参数校验、请求模型组装...
		return serviceCreateOrder(ctx)
	})
	return err
}

// serviceCreateOrder 是 service 层：本层处理代码（编排、幂等键等）写在调用 store 层之前。
func serviceCreateOrder(ctx context.Context) error {
	_, err := otelutil.WithSpan(ctx, "service.order.Create", func(ctx context.Context) error {
		// service 层处理：业务编排、幂等键、事务边界...
		return storeSaveOrder(ctx)
	})
	return err
}

// storeSaveOrder 是 store 层：本层处理代码（实体组装）写在调用 db 层之前。
func storeSaveOrder(ctx context.Context) error {
	_, err := otelutil.WithSpan(ctx, "store.order.Save", func(ctx context.Context) error {
		// store 层处理：实体组装、仓储约束...
		return dbInsertOrder(ctx)
	})
	return err
}

// dbInsertOrder 是 db 层：最底层 span，带 db.system 属性。
func dbInsertOrder(ctx context.Context) error {
	_, err := otelutil.WithSpan(ctx, "db.order.insert", func(ctx context.Context) error {
		// 模拟数据库写入；返回 nil 表示成功。
		return nil
	}, otelutil.WithStartOption(trace.WithAttributes(attribute.String("db.system", "mysql"))))
	return err
}
