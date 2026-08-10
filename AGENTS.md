# AGENTS.md — go-observability 开发规则

本文件是 go-observability 仓库的开发守则。任何 agent / 协作者改动本仓库前必须先读本文件。
本文件与 README.md、`docs/architecture.md`、`docs/configuration.md` 共同构成仓库内项目真源；文档与代码不一致时以本文件和代码为准，并在同一提交中修正文档。

## 项目身份

- 模块路径：`github.com/formal-you/go-observability`；Go 1.25。
- 定位：基于 OpenTelemetry 语义约定的语义化日志组件，采用「方案2 源即规范」——属性键直接用 OTel semconv 1.41.0 名 + `app.*` vendor 命名空间。
- 核心 log 包零外部依赖（仅标准库：log/slog、net、time、fmt、strings）；OTel 依赖只允许出现在 `internal/attrkv`、`writer/*`、`telemetry`、`middleware/*`（gin/errresp/recover/nethttp 用 OTel trace 提取 span context）、`example/*`。
- 采集频率归 opentelemetry-collector（batch processor）；核心层每次 Emit 同步写出，不做批处理/定时器。

## 防漂移硬规则（违反即视为实现漂移）

1. 事件名（EventName）须为经 `Validate` 的三段式常量，禁止在中间件/生产埋点里散落手写字符串。
   - **框架级**（access/error 中间件默认等）登记在核心 `types.go`（如 `EventNameAccessHTTPRequest`）。
   - **领域 business.*** 由接入方自建注册表（见 `example/mall`），可用 `NewEventName` 或包内 `const EventName = "business.…"`；不要把电商等领域名写进核心 `types.go`。
2. semconv 键名以 1.41.0 为准（核对 `$GOMODCACHE/go.opentelemetry.io/otel@v1.44.0/semconv/v1.41.0` 常量）：
   - 路径用 `url.path`（不是 `http.request.path`）；
   - 代码位置用 `code.function.name` / `code.file.path` / `code.line.number`（不是 `code.function` / `code.filepath` / `code.lineno`）；
   - `event.name` 走 LogRecord 的 EventName 顶层字段，不写属性（属性名 `otel.event.name` 仅桥接场景用）。
3. LogRecord 顶层字段映射由 `internal/attrkv.Record` 集中完成：`timestamp`→Timestamp、`level`→SeverityNumber/SeverityText、`event.name`→EventName；`trace_id`/`span_id` 不写属性，由 ctx 的 span context 自动关联（sdk/log Logger.Emit 行为，ginlog 传 `c.Request.Context()`）。新增保留键必须同时更新 `attrkv.recordAttrKeys` 与 `normalize.reservedKeys`。
4. 核心公共字段必须在 `keys.go` 登记；vendor 一律 `app.*`。领域专属键（order_id 等）由接入方自建，不进核心 `keys.go`（见 `example/mall`）。
5. 双投影：file/stdout 扁平列保留 `timestamp`/`level`/`trace_id`/`span_id`/`event.name` 等键（运营投影）；OTLP 路径剥离这些键。不要为 OTLP 把顶层字段塞回属性，也不要为 file 把属性拆掉。
6. schema / 键名 / 字段归属变更必须同步文档（README.md、detailed-design.md、方案2-直接符合规范.md），改代码与改文档应在同一提交内完成。
7. 零值省略：字符串/数值零值省略；布尔不省略（false 对 retryable/result 语义明确）。
8. samber 生态只允许出现在 `example/` 与 `docs/samber-comparison.md`，核心包保持零外部依赖。
9. 能力归属：库承载所有通用可复用能力（错误分类/投影、HTTP/gRPC/Gin 收口中间件、三信号装配），并通过配置或注入点（如 `ResponseProjector`）允许接入方自定义契约；接入方不得在应用侧镜像库中间件，契约差异一律经库的注入点表达。

## 开发流程

- 先回答用户问题，再执行修改。
- 改 Go 代码后必须运行（本机低内存组合）：
  - `gofmt -w <改动的文件>`
  - `$env:GOMAXPROCS=1; $env:GOGC=30; go build ./...; go vet ./...; go test ./...`
- 新增行为必须有测试覆盖（注册表校验、字段映射、剥离规则、writer 形状），并贴回完整输出作为验证证据。
- 注释标准库级：每条导出声明以标识符名开头，说明"是什么 + 为什么/怎么用"；**注释中文**，术语保留英文（span、trace、semconv 等）。
- 不要把跑 `--help` 或仅 build 当作验证；至少跑一次相关 `go test`。

## Git 纪律（2026-08-09 用户规范：每次改动必须 commit）

- 每次改动后必须 commit，不再等待用户要求；暂存用显式路径 `git add <path>`，禁止 `git add -A` / `git add .`。
- 提交信息用 conventional commits（feat/fix/docs: 简述）；一次提交尽量对应一个逻辑变更。
- 不 push、不 force push，除非用户明确要求。

## 目录速查

- 根包 log：`types.go`（枚举 + EventName 常量注册表）、`keys.go`（属性键常量）、`metadata.go`、`payload.go`（六类载荷）、`events.go`（六类事件结构体）、`normalize.go`（归一化 / 保留键）、`log.go`（Logger / Writer / 采样 / 脱敏接口）。
- `internal/attrkv/`：slog.Attr ↔ OTel 转换 + Record 顶层字段映射（唯一核心映射层）。
- `writer/{otlp,stdout,file}/`：后端 Writer（装配层，可替换）。
- `middleware/ginlog/`、`middleware/errresp/`、`middleware/recover/`：Gin 集成；`middleware/nethttp/`：net/http 错误收口；`middleware/metrics/`：HTTP/gRPC 服务器指标；`middleware/trace/`：HTTP/gRPC 服务器链路；net/http 示例见 example/nethttp。
- `errs/`：错误体系，零外部依赖；`error_project.go` 投影。
- `telemetry/`：对外公开的三信号装配与环境变量出口选择。
- `ResultKeepSampler` / `FieldMasker`：可选采样与脱敏实现；NewLogger 不自动挂载。
- `observability/`：LGTM 参考栈 + templates。
- `example/{mall,metrics,nethttp,samber}`：接入方示范。

## 常用真源

- 用户入口与公共承诺：`README.md`、`docs/`
- 开发与验证流程：`CONTRIBUTING.md`、`docs/workflow.md`
- semconv 1.41.0 常量：`$GOMODCACHE/go.opentelemetry.io/otel@v1.44.0/semconv/v1.41.0`
