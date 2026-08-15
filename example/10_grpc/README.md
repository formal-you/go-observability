# 10 · gRPC 拦截器

目标：演示 gRPC 的 server span 与 `rpc.server.duration` 两个 unary 拦截器。

从仓库根目录运行：

```powershell
go run ./example/10_grpc
```

完整服务需要 protoc 生成代码；本示例用 `grpc.NewServer` 展示真实装配方式，并用 `grpc.UnaryServerInfo` 直接调用拦截器，避免依赖生成代码也能跑通。设置 `OTEL_EXPORTER_OTLP_ENDPOINT` 后 Trace / Metric 改走 OTLP。

关键 API：`grpcmw.Trace` / `grpcmw.Metrics` / `grpc.ChainUnaryInterceptor`。
