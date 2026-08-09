# AGENTS.md — go-observability 开发规则

本文件是 go-observability 仓库的开发守则。任何 agent / 协作者改动本仓库前必须先读本文件。
本文件与 README.md、详细设计（observability-design/outline/B1-event-structs/ 下 detailed-design.md、方案2-直接符合规范.md）共同构成项目真源；三者不一致时以本文件与代码为准，并同步修正文档。

## 项目身份

- 模块路径：`github.com/formal-you/go-observability`；Go 1.25。
- 定位：基于 OpenTelemetry 语义约定的语义化日志组件，采用「方案2 源即规范」——属性键直接用 OTel semconv 1.41.0 名 + `app.*` vendor 命名空间。
- 核心 log 包零外部依赖（仅标准库：log/slog、net、time、fmt、strings）；OTel 依赖只允许出现在 `internal/attrkv`、`writer/*`、`internal/telemetry`、`middleware/ginlog`、`example/*`。
- 采集频率归 opentelemetry-collector（batch processor）；核心层每次 Emit 同步写出，不做批处理/定时器。

## 防漂移硬规则（违反即视为实现漂移）

1. 事件名（EventName）必须使用 `types.go` 的常量注册表（如 `EventNameAccessHTTPRequest`），禁止在载荷、中间件、示例、文档里手写 `"access.http.request"` 这类字符串。新增事件名必须：在 `types.go` 登记常量、符合段式 `类别.模块.操作`（3 段，仅小写字母/数字/下划线，可用 `NewEventName`/`Validate` 校验）、同步 detailed-design.md §2。
2. semconv 键名以 1.41.0 为准（核对 `$GOMODCACHE/go.opentelemetry.io/otel@v1.44.0/semconv/v1.41.0` 常量）：
   - 路径用 `url.path`（不是 `http.request.path`）；
   - 代码位置用 `code.function.name` / `code.file.path` / `code.line.number`（不是 `code.function` / `code.filepath` / `code.lineno`）；
   - `event.name` 走 LogRecord 的 EventName 顶层字段，不写属性（属性名 `otel.event.name` 仅桥接场景用）。
3. LogRecord 顶层字段映射由 `internal/attrkv.Record` 集中完成：`timestamp`→Timestamp、`level`→SeverityNumber/SeverityText、`event.name`→EventName；`trace_id`/`span_id` 不写属性，由 ctx 的 span context 自动关联（sdk/log Logger.Emit 行为，ginlog 传 `c.Request.Context()`）。新增保留键必须同时更新 `attrkv.recordAttrKeys` 与 `normalize.reservedKeys`。
4. 新增/修改字段必须在 `keys.go` 登记常量；vendor 字段一律 `app.*` 前缀，不与 semconv 冲突。
5. 双投影：file/stdout 扁平列保留 `timestamp`/`level`/`trace_id`/`span_id`/`event.name` 等键（运营投影）；OTLP 路径剥离这些键。不要为 OTLP 把顶层字段塞回属性，也不要为 file 把属性拆掉。
6. schema / 键名 / 字段归属变更必须同步文档（README.md、detailed-design.md、方案2-直接符合规范.md），改代码与改文档应在同一提交内完成。
7. 零值省略：字符串/数值零值省略；布尔不省略（false 对 retryable/result 语义明确）。
8. samber 生态只允许出现在 `example/` 与 `docs/samber-comparison.md`，核心包保持零外部依赖。

## 开发流程

- 先回答用户问题，再执行修改。
- 改 Go 代码后必须运行（本机低内存组合）：
  - `gofmt -w <改动的文件>`
  - `$env:GOMAXPROCS=1; $env:GOGC=30; go build ./...; go vet ./...; go test ./...`
- 新增行为必须有测试覆盖（注册表校验、字段映射、剥离规则、writer 形状），并贴回完整输出作为验证证据。
- 注释标准库级：每条导出声明以标识符名开头，说明"是什么 + 为什么/怎么用"；注释中文，术语保留英文（span、trace、semconv 等）。
- 不要把跑 `--help` 或仅 build 当作验证；至少跑一次相关 `go test`。

## Git 纪律（2026-08-09 用户规范：每次改动必须 commit）

- 每次改动后必须 commit，不再等待用户要求；暂存用显式路径 `git add <path>`，禁止 `git add -A` / `git add .`。
- 提交信息用 conventional commits（feat/fix/docs: 简述）；一次提交尽量对应一个逻辑变更。
- 不 push、不 force push，除非用户明确要求。

## 目录速查

- 根包 log：`types.go`（枚举 + EventName 常量注册表）、`keys.go`（属性键常量）、`metadata.go`、`payload.go`（六类载荷）、`events.go`（六类事件结构体）、`normalize.go`（归一化 / 保留键）、`log.go`（Logger / Writer / 采样 / 脱敏接口）。
- `internal/attrkv/`：slog.Attr ↔ OTel 转换 + Record 顶层字段映射（唯一核心映射层）。
- `writer/{otlp,stdout,file}/`：后端 Writer（装配层，可替换）。
- `middleware/ginlog/`：Gin 中间件（otelgin trace 提取、access 事件）。
- `errs/`：错误体系（ErrorKind / ErrorType v1 / AppError / BizError / SystemError / StackRule / CaptureSource / CaptureStack），零外部依赖；根 log 包经 error_project.go 的 EventFromError 投影。
- `internal/telemetry/`：三信号装配（trace/metric/log provider + OTLP gRPC 导出 + A3 采样/频率 + A7 资源属性；env: OTEL_SDK_DISABLED / OTEL_EXPORTER_OTLP_ENDPOINT）。
- `middleware/recover/`：Gin panic 收口中间件（runtime.panic ErrorEvent + 统一 500，StackRule=must 必记堆栈）。
- `observability/`：LGTM 参考栈（docker-compose + otel-collector-config + tempo/loki/mimir + grafana provisioning），Trace/Metric/Log 部署侧参考配置。
- `example/`：演示与 spike（samber 对照实验见 `example/samber`）。

## 常用真源

- 详细设计：`observability-design/outline/B1-event-structs/detailed-design.md`
- 符合性方案：`observability-design/outline/B1-event-structs/方案2-直接符合规范.md`（采用）、`方案1-转换器.md`（备选）
- semconv 1.41.0 常量：`$GOMODCACHE/go.opentelemetry.io/otel@v1.44.0/semconv/v1.41.0`
