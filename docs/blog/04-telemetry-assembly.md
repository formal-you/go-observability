# 04 · 三信号统一装配：Trace、Metric、Log 不再各起炉灶

## 一、痛点

很多项目里，Trace 用一套初始化，Metric 用一套初始化，Log 又一套初始化：

- `service.name` 可能不一致；
- `Shutdown` 顺序可能出错；
- 环境变量各自读取，部署时容易漏配。

`go-observability` 用 `telemetry.Runtime` 统一装配三个信号，并按 `Log -> Metric -> Trace` 的顺序关闭。

## 二、四种预设

| 预设 | Trace | Metric | Log | 适合场景 |
| --- | --- | --- | --- | --- |
| `NewFileRuntime` | local | none | file | 小单体、无需 Collector |
| `NewLogRuntime` | none | none | file/otlp/stdout | 只要日志 |
| `NewOTLPRuntime` | otlp | otlp | otlp | 已有 Collector |
| `NewAllFileRuntime` | file | file | file | 完全离线排查 |

## 三、动手

从仓库根目录运行：

```powershell
go run ./example/07_telemetry -mode=file
go run ./example/07_telemetry -mode=log
go run ./example/07_telemetry -mode=all-file
```

```bash
go run ./example/07_telemetry -mode=file
go run ./example/07_telemetry -mode=log
go run ./example/07_telemetry -mode=all-file
```

OTLP 模式需要 Collector：

```powershell
go run ./example/07_telemetry -mode=otlp -endpoint=127.0.0.1:4317
```

## 四、需要自定义时用 `NewRuntime`

预设只能覆盖常见场景。需要自定义采样比例、批量导出间隔、队列或轮转时：

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
    Enabled:  telemetry.EnabledFromEnvironment(),
    Endpoint: telemetry.EndpointFromEnvironment(),
    Resource: telemetry.ResourceConfig{ServiceName: "order-api"},
    Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputOTLP, SampleRatio: 1},
    Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputOTLP},
    Log:      telemetry.LogConfig{Output: telemetry.SignalOutputOTLP},
})
```

资源所有权是统一的：

```text
InstallGlobal -> NewWriter -> Emit -> Close Writer -> Shutdown Runtime
```


## 代码对比：三信号初始化

传统做法要分别维护三个 Provider：

```go
tp := initTraceProvider()
mp := initMeterProvider()
lp := initLoggerProvider()
```

`go-observability`：

```go
runtime, err := telemetry.NewRuntime(ctx, telemetry.Config{
    Enabled:  true,
    Endpoint: "otel-collector:4317",
    Resource: telemetry.ResourceConfig{ServiceName: "order-api"},
    Trace:    telemetry.TraceConfig{Output: telemetry.SignalOutputOTLP},
    Metric:   telemetry.MetricConfig{Output: telemetry.SignalOutputOTLP},
    Log:      telemetry.LogConfig{Output: telemetry.SignalOutputOTLP},
})
restore := runtime.InstallGlobal()
defer restore()
```

三信号共享一个 `Resource` 和一个 `Shutdown`，初始化/关闭顺序由 Runtime 统一管理。

## 五、最佳实践

1. `service.name` 只在 `Resource` 配置一次，三信号共用；
2. `OTEL_SDK_DISABLED` 用于离线/测试，`OTEL_EXPORTER_OTLP_ENDPOINT` 用于切换出口；
3. 不要在请求热路径里反复构造 Runtime，进程启动期构造一次；
4. 退出前先 Close Writer，再 Shutdown Runtime。

下一篇：[05 · 框架中间件接入](05-framework-middleware.md)。
