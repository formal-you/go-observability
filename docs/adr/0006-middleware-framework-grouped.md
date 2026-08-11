# ADR-0006：中间件按框架体系分组（gin / http / grpc / kratos）

- 状态：Accepted
- 日期：2026-08-11
- 关联：ADR-0005（httperr 契约核心）

## 背景（Context）

middleware 目录按「关注点」平铺命名，框架归属不直观：errresp / recover 看不出是 Gin 专属；
trace / metrics 一个包同时服务 Gin、net/http、gRPC 三个框架（共用）；外部接入方无法
「一看就知道某个包的中间件属于哪个框架体系」。用户要求按框架体系组织。

## 决策（Decision）

- 目录按框架体系分组：
  - `middleware/gin/`（ginmw）：Gin 体系——AccessLog / ErrorResponse / Recover / Trace / Metrics / Abort；
  - `middleware/http/`（httpmw）：net/http 体系——ErrorResponse / Recover / SetError / Trace / Metrics；
  - `middleware/grpc/`（grpcmw）：gRPC 体系——Trace / Metrics unary 拦截器；
  - `middleware/kratos/`（kratosmw）：kratos v3 传输适配（保持不变）；
  - `middleware/httperr/`：框架无关错误契约核心（保持不变）；
  - `middleware/otelutil/`：框架无关 OTel 工具（链路注入/提取、TraceExtractor）；
  - `middleware/internal/mwutil/`：内部共享底层工具（状态码记录、路由/RPC 解析、span 收尾、直方图）。
- 原 ginlog / errresp / recover / nethttp / metrics / trace 六个包删除；函数按新命名收敛
  （如 ginlog.Middleware → ginmw.AccessLog、errresp.Middleware → ginmw.ErrorResponse、
  trace.NewHTTPMiddleware → httpmw.Trace 等），Config 拆为按功能命名（AccessConfig / ErrorConfig / ...）。
- 工作区依赖方同步适配：mall、ai-gateway（import 路径 + 符号名）；kratos-mall 仅用 kratos 包，无需改。

## 被否方案

- 保留按关注点平铺、仅给包加框架前缀（ginerrresp / ginrecover / httptrace / grpctrace）：
  命名冗长且 trace/metrics 的「一包多框架」共用问题依旧，框架体系不直观。
- 拆成 middleware/gin/errresp 等多层子包：目录过深、单文件包过多，不符合 Go 惯例。

## 结果（Consequences）

- 正面：包名即框架体系，接入方按框架选目录；trace/metrics 按框架收敛，消除「一包多框架」；
  httperr / otelutil / mwutil 独立承载共享逻辑，避免重复。
- 代价：破坏性变更——六个旧包删除，导入路径与符号名全部迁移；外部消费者（mall /
  ai-gateway）与 example 同步更新；文档（architecture.md / AGENTS.md）与 ADR-0005 的
  目录表述一并更新。
- 兼容：行为与事件/响应契约不变（迁移后既有黑盒测试原样通过）。
