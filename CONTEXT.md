# go-observability 领域上下文

go-observability 是 Go 语义化日志与可观测装配库：定义事件如何分类、命名、治理（采样/脱敏）与输出，并统一 Trace / Metric / Log 三信号装配的术语。本文件是术语表（ubiquitous language），不是规格或实现说明；术语冲突时以本文件为准。

## Language

### 事件模型

事件模型分三层：
① **Event Semantics**：event.name 唯一标识 Event Structure（发生了什么）；
② **Error Semantics**：失败/异常事件的 error.type（OTel/gRPC 标准分类）+ error.code（企业级具体错误）；
③ **Sampling / Retention**：高价值事件（failed/error/blocked/denied、security 等）的采样优先级与保留策略，独立于①②，不编码进 event.name / error.type 语义。

**事件（Event）**:
一次语义化日志记录，由粗分类（EventType）、细名（EventName）与扁平属性组成，经 Logger 治理后由 Writer 输出。
_Avoid_: 日志行、log message（指结构化事件时）

**EventType**:
事件的粗分类，固定六类：access / business / error / security / audit / probe；随事件写为 type。
_Avoid_: 日志级别（Level 是另一维度）、EventName

**EventName**:
事件的稳定事实名，event.name MUST use the form <domain>.<subject>.<event>（如 http.request.completed、order.payment.succeeded）；<event> MUST 是注册的 Event Type（稳定语义发生，唯一标识 Event Structure），不是自由文本，也不是 Operation Lifecycle Stage（生命周期经 Span 建模）；输入键 event.name（正则 EventNamePattern 校验）；OTLP 投影为顶层 LogRecord.EventName（不进入 attributes），file/stdout 双投影保留扁平 event.name 列；粗分类已由 type/EventType 承载，首段不得重复六类前缀。
_Avoid_: 类别.模块.操作、随意字符串、生命周期 Stage、EventType 前缀

**ErrorType**:
低基数失败类别，映射 OTel error.type，复用 OTel/gRPC 标准枚举（gRPC canonical code）：跨模块、闭合、单段 UPPER_SNAKE（如 NOT_FOUND / UNAVAILABLE / DEADLINE_EXCEEDED / INTERNAL）；以通用性换取工具链兼容与低维护成本，禁止自定义词表。
_Avoid_: ErrorCode（高基数具体码）、error message

**Error Registry**:
errs 内 ErrorCode → ErrorType 的固定映射注册表（多对一：一个 code 恰好一个 type，多个 code 可共用一个 type），严格构造器对已注册码强制类型一致；注册是接入方显式选择，未注册码不强制。
_Avoid_: 自动派生、运行时可变映射

**ErrorCode**:
可选的稳定具体错误码，规范文法为 `err.code = SCOPE.OPERATION.REASON`，对应（服务/模块）.（场景/操作）.（结果/具体错误）；每段只允许大写字母、数字和下划线（正则 `ErrorCodePattern` 校验）。第三段统一为 REASON，不因业务/系统分裂成 cause/reason 两套命名；业务/系统区分由 error.type 承担。SCOPE 归属由失败面决定：业务/校验拒绝用业务模块（ORDER/PAYMENT/USER…），基础设施故障用 `INFRA.*`（OPERATION=组件，如 `INFRA.REDIS.UNAVAILABLE`），非基础设施系统类可保留领域 SCOPE；INTERNAL/panic 不承载 error.code（定位靠 error.type + event.name + stacktrace）。event.name 与 error.code 前两段只做软对齐（SHOULD）：业务码尽量与 domain.subject 同源，INFRA.* 与业务事件名天然不同源。依赖名由 `app.upstream_service` 属性承载，不占用 error.code 段位（ADR-0019）。它用于客服、业务查询和精确聚合，统一写入 error.code；不决定 EventName，也不替代低基数 error.type。已注册的 ErrorCode 恰好映射一个 ErrorType（多对一，见 Error Registry）。

```text
业务错误：
  event.name = <domain>.<subject>.<event>
  error.code = <DOMAIN>.<SUBJECT>.<REASON>

基础设施错误：
  event.name = <domain>.<subject>.<event>
  error.code = INFRA.<COMPONENT>.<REASON>
```

_Avoid_: app.business_code、app.operation、依赖名直接当 SCOPE（如 REDIS.CONNECTION_POOL_EXHAUSTED）、从错误码自动生成 EventName

**Level**:
语义化级别 DEBUG / INFO / WARN / ERROR，映射 slog.Level 与 OTel SeverityNumber。
_Avoid_: HTTP 状态码

**Result**:
跨事件类型的业务结果（vendor 键 app.result，保留作为跨事件直接过滤/采样列）：success / failed / error / blocked / denied / unknown；失败语义另由 error.type 表达。
_Avoid_: HTTP 状态码（状态码是访问事件的传输字段，不等于 Result）

**高价值结果（High-value Result）**:
failed / error / blocked / denied 四类，属于高价值失败/异常事件；由独立的 Sampling/Retention Policy 高优先级保留（SHOULD be retained，操作上需要时保证保留），不编码进 event.name / error.type 语义。
_Avoid_: 错误（仅指 error 一类）、异常

**安全/审计事件并存（Security/Audit Co-occurrence）**:
错误事件（ErrorEvent）每请求唯一（错误出口唯一）；经 InputGuard 注入的安全事件
（SecurityEvent）与审计事件（AuditEvent）可与错误事件并存，三者记录视角不同——
故障排障 / 安全聚合 / 合规审计，不是对同一失败的重复记录。
_Avoid_: 用多条错误事件表达同一失败

### 数据治理

**Sampling / Retention Policy（采样保留策略）**:
独立于事件/错误语义的策略层：高价值失败/异常事件（failed/error/blocked/denied）SHOULD be retained by the telemetry pipeline；sampling policy SHOULD prioritize or guarantee retention where operationally required；不把采样策略编码进 event.name / error.type 定义。
_Avoid_: 把「100% 保留」写进 Event Semantic Convention、用采样描述事件语义

**采样（Sampling）**:
按策略选择哪些事件保留、哪些丢弃的选择性减量；采样率（如 0.1）表示保留比例。
_Avoid_: 采集频率、导出频率、批处理

**批量导出间隔（Batch Export Interval）**:
SDK 侧多久打包导出一次（trace 5s / metric 15s / log 1s）与 Collector 侧 batch.timeout（凑批等待）；只影响时延与批大小，不丢数据。
_Avoid_: 采样、刷新频率、采集频率

**采集频率（避免使用）**:
仓库不采用此模糊词；精确说法是「批量导出间隔」（多久发一次）或「采样率」（留哪些）。
_Avoid_: 用它描述导出或采样

**头部采样（Head Sampling）**:
SDK 在导出前按 trace id 概率采样（TraceSampleRatio）；被丢弃的 trace 永远到不了 Collector，尾部采样无法恢复。
_Avoid_: 前端采样

**尾部采样（Tail Sampling）**:
Collector 侧等 trace 完整到达后按错误 / 概率 / 属性策略决定保留。
_Avoid_: 后端采样（模糊）

**Sampler**:
采样判定器，返回保留或丢弃；ResultKeepSampler 按 Result 高优先级保留（SHOULD），EventTypeKeepSampler 按 type/EventType 粗分类判定，EventKeepSampler 按 event.name 领域前缀判定。
_Avoid_: 采样频率、过滤器

**Masker（脱敏）**:
写出前按键名递归替换敏感值（覆盖 group / map / slice / LogValuer）。
_Avoid_: 加密、匿名化

**Writer**:
日志输出后端（file / stdout / otlp），位于治理管线末端，看到的是已完成采样与脱敏的扁平事件。
_Avoid_: Exporter（不同层）

### Trace 信号

**Trace**:
一次端到端请求的全链路记录，由多个 Span 组成父子树，共享同一 trace_id。
_Avoid_: 请求日志（单条日志不是 Trace）、日志链路

**Span**:
Trace 中的一个操作单元（如一次 HTTP 请求处理、一次 DB 调用），有 span_id、开始/结束时间与父子关系。
_Avoid_: 步骤、日志记录

**SpanContext**:
跨进程传递的链路标识：trace_id + span_id + trace flags + trace state，由 W3C Trace Context 承载；进程内则由 Go context.Context 携带当前 Span（含 SpanContext）在中间件与调用层间流转，用于耗时排查与错误定位。
_Avoid_: 把 SpanContext 当作完整 Span（进程内 context 携带的是当前 Span，SpanContext 只是其不可变标识）

**TraceIDRatioBased**:
按 trace_id 哈希做概率头部采样的采样器；TraceSampleRatio=0.1 表示保留约 10% 的 trace。
_Avoid_: 随机丢弃每条日志（它按 trace 整体采样，保证链路完整）

**ParentBased**:
父子一致的采样决策：父 Span 已采样则子继承，否则按自身采样器决定；保证同一 trace 内采样一致。
_Avoid_: 无父场景下的简单概率采样

**传播（Propagation）**:
把 SpanContext 与 Baggage 跨服务传递的机制，载体为 W3C Trace Context（traceparent / tracestate）与 Baggage 头。
_Avoid_: 传 header（header 只是载体）

**Baggage**:
随传播上下文传递的键值对，可被下游读取，但不自动进入 span 属性。
_Avoid_: 日志属性（属性是显式记录的）

**根 Span / 子 Span（Root / Child Span）**:
根 Span 无父节点，代表一次请求的入口；子 Span 挂在父 Span 之下。
_Avoid_: 主请求 / 次请求

### Metric 信号

**Metric（指标）**:
可聚合的数值观测（如请求延迟、QPS），由名称、单位与属性维度组成。
_Avoid_: 统计、监控数据（宽泛）

**Counter / UpDownCounter / Histogram / Gauge**:
OTel 四类指标工具：Counter 只增累计；UpDownCounter 可增减；Histogram 记录分布（本项目用 http.server.request.duration）；Gauge 记录瞬时值。
_Avoid_: 请求数（Counter 是累计计数，不是瞬时 QPS）

**PeriodicReader**:
SDK 侧周期读取并导出指标的机制（本项目默认 15s）；只决定导出时机，不丢数据。
_Avoid_: 采样器、聚合器

**Temporality（时间性）**:
指标的时间语义：delta（本周期增量）或 cumulative（自启动起的累计值）。
_Avoid_: 时间窗口（模糊）

### Log 信号（OTel）

**LogRecord**:
OTel 日志的最小单元，顶层字段含 Timestamp / Severity / SeverityText / Body / EventName；span context 由 ctx 自动关联，不写属性。
_Avoid_: 日志行字符串（LogRecord 是结构化记录）

**Severity（严重级别）**:
LogRecord 的 OTel 严重度（DEBUG/INFO/WARN/ERROR），由本项目 level 属性映射而来。
_Avoid_: level（项目内 level 是语义级别，Severity 是其 OTLP 形态）

**Body**:
LogRecord 的正文，本项目写入事件类型（event_type）；事件字段进属性，不塞进 Body。
_Avoid_: 完整事件内容

### 错误领域

**错误分类（ErrorKind）**:
错误预期性分类：validation / business / system；前两者是预期内拒绝（正常响应，不告警），system 是非预期故障（需要告警与重试评估）。
_Avoid_: 日志级别（WARN/ERROR 是 Level 维度）、错误码

**低基数失败类别（ErrorType）**:
error.type 的低基数失败类别，复用 OTel/gRPC 标准枚举（gRPC canonical code，跨模块、闭合、单段 UPPER_SNAKE，如 NOT_FOUND / UNAVAILABLE / DEADLINE_EXCEEDED / INTERNAL），用于聚合与告警路由。
与业务错误码（ErrorCode）是「宏观分类 > 微观错误码」的多对一关系：多个三段式
ErrorCode 归入同一个标准枚举 ErrorType，ErrorType 不是对 ErrorCode 的细化。
_Avoid_: 错误消息（消息是高基数文本，类别是低基数标签）

**业务错误码（ErrorCode）**:
业务错误码：（服务/模块）.（场景/操作）.（结果/具体错误）（如 ORDER.CREATE.STOCK_INSUFFICIENT）；由 BizError 承载，SystemError 可经 WithCode 关联可选码。
_Avoid_: ErrorType、error message

**错误投影（Error Projection）**:
把 errs.AppError 按 Kind 映射为 BusinessEvent / ErrorEvent 的字段投影（EventFromError），不负责写出、采样与告警判定。
_Avoid_: 序列化、格式化

**重试上下文（Retry Context）**:
system 错误的重试元数据：retryable、retries、retries_exhausted、upstream。
_Avoid_: 重试逻辑（本仓库只记录元数据，不执行重试）

**堆栈策略（StackPolicy）**:
堆栈记录策略：must（构造点必记）/ optional（按需）/ none（不记），按 error.type 精确 code 生效，控制体积。
_Avoid_: 一律记堆栈（会让高频类别体积失控）

**堆栈边界（StackConfig）**:
为 stacktrace 设置最大 UTF-8 字节数、路径策略（full/base/redacted）与 ErrorType 覆盖。超限使用稳定截断标记并输出 `app.stacktrace_truncated=true`；`INTERNAL`（含 panic）不得被降为 none。

**Subject / Actor**:
Subject 是事件关联的用户与租户，输出为 `user.id` / `app.tenant_id`；Actor 是安全或审计动作的执行者，输出为 `app.actor_user_id` / `app.actor_role`。可信认证上下文通过 `IdentityExtractor` 注入，非空值优先于事件载荷；二者都不是 Metric 维度。

**输入摘要（Input Summary）**:
非法输入的关键字段名 / 哈希 / 截断文本，由接入方提取并经 `httperr.WithInputSummary`
挂到 ctx，供错误收口的 `InputGuard` 读取；只记 `app.*` 摘要，不落原始 body。
_Avoid_: 原始请求 body / 完整 payload（高基数且可能含 PII/凭证）

### OTel 基础设施与协议

**信号（Signal）**:
OTel 的三大数据类型：trace / metric / log。
_Avoid_: 数据源、Writer

**SDK 与 API**:
OTel 分层：API 定义接口与类型，SDK 提供实现（Provider / Exporter / Reader）；本项目 log 包只依赖 slog 标准库，OTel SDK 集中在 telemetry / writer / middleware。
_Avoid_: 混用两层的概念

**OTLP**:
OTel 的传输协议（gRPC / HTTP），把三信号发送给 Collector 或后端；本项目默认 127.0.0.1:4317。
_Avoid_: 数据库连接（OTLP 是协议，不是存储）

**Collector**:
独立进程，接收 OTLP 后处理（batch、tail_sampling 等）再转发到后端（Tempo / Loki / Mimir）。
_Avoid_: 数据库、日志存储

**Attribute（属性）**:
事件 / Span / Metric 上的键值元数据，键名遵循 semconv；Resource 是服务身份，不属于事件属性。
_Avoid_: Resource、随意字段名

**semconv（语义约定）**:
OTel 语义约定：规范化属性名、单位与命名（如 http.response.status_code）；本项目以 1.41.0 为准。
_Avoid_: 私有键、自造键名

**Instrumentation（插桩）**:
在应用中埋点产生信号；本项目以中间件形式提供（ginlog / metrics / trace / recover 等）。
_Avoid_: 日志打印（埋点是结构化信号产生）

### 遥测与装配

**TraceExtractor**:
从 ctx 提取当前 Span 链路标识的适配器（经 WithTraceExtractor 注入）；事件未显式携带 trace_id/span_id 时自动补全，不覆盖已设置值。
_Avoid_: 传播（跨服务传递）——TraceExtractor 是进程内补全

**层（SDK 侧 / Collector 侧）**:
频率决策分两层：应用 SDK 决定采样率与批量导出间隔；Collector 只做接收端 batch 凑批与可选尾部采样，管不到 SDK 何时导出。
_Avoid_: 用 Collector 配置描述 SDK 采样率或导出间隔

**Exporter**:
telemetry 侧的 OTel 导出器（otlptrace / otlpmetric / otlplog），与 log 包 Writer 不同层。
_Avoid_: Writer

**Provider**:
trace / metric / log 三信号 Provider 的装配与全局安装（otel / global 包）。
_Avoid_: Exporter

**Resource**:
服务身份与低基数标签（service.name、service.version、service.instance.id、deployment.environment.name、region）。
_Avoid_: 属性（attribute 是事件字段维度）

**Pipeline（管线）**:
Collector 中 receiver → processor → exporter 的链路；本仓库三信号各一条（traces / metrics / logs）。
_Avoid_: 数据流图、Writer

### 输出与归一化

**双投影（Dual Projection）**:
同一事件面向两个正确形状：file/stdout 扁平运营列（timestamp / level / trace_id / event.name…），OTLP 映射 LogRecord 顶层字段（Severity / EventName / Timestamp / span context）。
_Avoid_: 一份数据两种格式（会误解为重复数据）

**归一化（Normalization）**:
metadata + payload 合并为扁平 attrs，过滤保留键与零值省略。
_Avoid_: 序列化（序列化在 Writer 层）

**心跳 / 健康检查（Heartbeat / Health Check）**:
确定性高频噪音（如 /healthz），用 SkipPaths 或接入层短路排除，不进入概率采样。
_Avoid_: 访问事件（心跳不是业务访问）
