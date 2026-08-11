# PR Proposal：Production Observability Event System

- 状态：Draft，待确认关键决策后实施
- 日期：2026-08-11
- 输入：`example/blackbox/sample.json`、当前代码、`CONTEXT.md`、既有 ADR
- 范围：定义后续 PR 的行为契约与拆分，不在本文提交中修改生产代码

## 1. 结论

认可“把应用日志升级为生产级 Observability Event System”的目标，但不接受原优化计划原样落地。
原计划只根据 file 投影样例判断能力，混淆了 OTel Resource、LogRecord 顶层字段、事件属性和
Trace 父子关系，并使用了部分过时或非规范字段名。

本仓库继续采用既定的“方案 2：源即规范”：

- 事件属性保持扁平，使用 OTel semconv 1.41.0 与 `app.*` vendor namespace；
- OTLP 的服务身份放在 Resource，file 运营投影按查询需要展开选定 Resource 字段；
- `event.name`、`error.type`、具体错误码各自回答不同问题，不组成嵌套 `error` 对象；
- Trace 的父子树由 Span 数据表达，日志只关联当前 `trace_id` / `span_id`；
- 核心 Logger 保持同步治理，OTLP 的异步批量导出继续由 SDK 与 Collector 负责。

## 2. 样例事实

`sample.json` 中共 12 条事件。它已经证明：

- 12/12 有合法 `trace_id` / `span_id`；
- 10 条 HTTP 来源事件具有 `request_id`，2 条后台事件没有伪造 `request_id`；
- BusinessEvent success、业务拒绝、系统错误、panic、安全和审计事件均可关联 AccessEvent；
- HTTP 字段只属于 AccessEvent；
- 系统错误按既有 StackPolicy 输出堆栈。

它同时暴露了三个真实问题：

- 只有 5/12 条 file 事件包含 `timestamp`；
- file 投影没有服务身份，无法仅凭文件查询服务、版本、实例和环境；
- 业务拒绝是 `msg=business`，但默认 `event.name=error.http.request`，粗分类和细名不一致。

`sample.json` 不是 OTLP 完整视图。当前 telemetry 已把服务信息放在 OTel Resource，因此“样例没有
service 字段”等于“file 投影不可见”，不等于“OTLP 没有 Service Metadata”。

此外，`sample.json` 是多个格式化 JSON 对象直接串接，不是合法的单个 JSON 文档，也不是每行一个
对象的 JSONL；它只适合人工审阅。机器验收与采集应使用程序覆盖生成的 `sample.jsonl`。

## 3. 对原计划的裁决

| 原建议 | 裁决 | 修订后的契约 |
| --- | --- | --- |
| Service Metadata 自动注入 | 部分认可 | OTLP 已有 Resource；修正 semconv 键，并让 file 运营投影可查询 |
| request_id → trace_id → span_id | 不认可层级关系 | 三者用途不同；HTTP 请求只要求稳定关联，不定义父子关系 |
| 增加 parent_span_id | 不认可 | 父 Span 属于 Trace 数据；重复写入每条日志会产生冗余和漂移 |
| 重构 error schema | 认可问题，修订字段 | 保留 `error.type`；具体码使用 `app.error_code`；消息按业务/异常语义分开 |
| 生产默认不写完整 stacktrace | 部分认可 | 已有 StackPolicy；增加大小、脱敏和生产 profile 约束，不在核心内建对象存储 |
| 增加 user.id / tenant.id | 能力已部分存在 | 当前已有 `app.user_id` / `app.tenant_id`；后续统一上下文注入与 PII 策略 |
| 改为嵌套 service/http/error 对象 | 不认可 | 会破坏源即规范、OTel 映射和现有字段查询；继续使用扁平键 |
| 日志 Writer 全部异步 | 不认可默认化 | OTLP 已由 BatchProcessor 异步批量导出；file 仍同步，异步 file 必须显式选择溢出策略 |
| 增强 OpenTelemetry 兼容 | 已是基线 | 本阶段是修复语义漂移，而不是重新引入 OTel |
| 采样与高并发优化 | 认可为后续项 | 保留默认全量 AccessEvent；通过基准和显式策略优化，不改默认契约 |

原计划中的 `http.method`、`http.status_code`、`deployment.environment` 和普通 `instance` 不作为
新契约。对应的 1.41.0 键应为：

- `http.request.method`；
- `http.response.status_code`；
- `deployment.environment.name`；
- `service.instance.id`。

## 4. 规范术语

| ID | 术语 | 本提案中的唯一含义 |
| --- | --- | --- |
| TERM-LOG-001 | Service Resource | 进程/服务级身份，OTLP 中属于 Resource，不是每条 LogRecord 的业务属性 |
| TERM-LOG-002 | EventType | 六类粗分类，file 的 `msg`、OTLP 的 Body |
| TERM-LOG-003 | EventName | 三段式具体事件名，首段必须与 EventType 一致 |
| TERM-LOG-004 | ErrorType | `error.type`，可预测、低基数的失败类别，用于聚合和告警 |
| TERM-LOG-005 | ErrorCode | 具体、可能高基数的稳定错误码，不作为 Metric 维度 |
| TERM-LOG-006 | RequestID | 面向用户、客服和单次入口请求的报障凭证 |
| TERM-LOG-007 | TraceID | 一次端到端 Trace 的关联键，跨服务传播 |
| TERM-LOG-008 | SpanID | 当前操作 Span 的标识；父子关系由 Trace 信号维护 |

以下关系是并列关联，不是父子模型：

```text
HTTP request
├── request_id   用户/客服查询
└── trace_id     端到端链路
    └── span_id  当前操作

Trace storage
└── Span(parent_span_id -> span_id)  仅由 Trace 表达父子树
```

## 5. P0：语义正确性与基础查询

### RULE-P0-001：所有投影都有事件时间

- Logger 在统一归一化阶段为零值 Timestamp 补当前时间；
- file、stdout、OTLP 对同一事件使用同一个 Timestamp；
- AccessEvent 可继续使用请求开始时间，但其他事件不能缺时间；
- 黑盒样例的 12/12 条事件必须包含可解析的 RFC3339Nano 时间。

### RULE-P0-002：Service Resource 双投影

- telemetry 启用时 `service.name` 必填；
- Resource 使用 `service.name`、`service.version`、`service.instance.id`、
  `deployment.environment.name`；
- OTLP 只在 Resource 保存这些字段，不重复塞回 LogRecord Attributes；
- file 运营投影把上述选定字段展开为扁平顶层键，支持 OpenSearch/Loki 直接查询；
- 库不猜测生产实例 ID，实例值由部署配置、主机名或 Pod UID 明确注入。

### RULE-P0-003：EventName 与 EventType 一致

- `EventName` 首段必须等于 EventType；
- 业务拒绝默认使用 `business.http.rejected`，系统错误使用 `error.http.request`；
- 领域应用可通过库提供的 resolver 注入更具体的 `business.order.rejected`；
- 框架中间件不得根据 error message 猜测领域事件名。

### RULE-P0-004：错误字段各司其职

推荐扁平模型：

```json
{
  "msg": "business",
  "event.name": "business.order.rejected",
  "error.type": "business.stock_insufficient",
  "app.error_code": "ORDER.CREATE.STOCK_INSUFFICIENT",
  "app.business_message": "商品库存不足",
  "app.result": "failed"
}
```

- `event.name` 描述发生了什么；
- `error.type` 描述低基数失败类别；
- `app.error_code` 描述具体稳定错误码；OTel 1.41.0 没有本项目可直接采用的通用 `error.code`；
- 业务拒绝使用 `app.business_message`，系统异常使用 `exception.message`；
- `exception.type` 只在能稳定提取实际异常类型时输出，不用它替代 `error.type`；
- 现有 `app.business_code` 和 SystemError→`app.operation` 的迁移必须单独写兼容说明。

### RULE-P0-005：ID 关联契约

- HTTP 来源的 Access/Business/Error/Security/Audit 事件共享 `request_id` 与 `trace_id`；
- 业务代码创建子 Span 时允许 `span_id` 不同；
- 后台任务可以有 trace/span，但没有 HTTP 请求时不生成 `request_id` 或 AccessEvent；
- `request_id` 优先接受可信网关值，否则由库的可注入 Generator 生成；
- 不新增日志字段 `parent_span_id`。

## 6. P1：安全上下文与堆栈治理

### RULE-P1-001：用户与租户上下文

- 保留统一 Subject/Actor 模型，避免业务代码在每条事件上手工拼接；
- 评估把用户标识对齐为 `user.id`，租户继续使用 `app.tenant_id`；
- user/tenant 只用于日志与 trace 查询，禁止作为默认 Metric 维度；
- 默认敏感键覆盖 password、token、authorization、cookie、证件号、手机号等；
- 原始凭证和原始 request body 不进入事件。

### RULE-P1-002：StackPolicy 有界化

- 保留 `exception.stacktrace` 标准字段与现有 must/optional/none 分类；
- 增加最大字节数、截断标记和路径处理策略，避免单条日志失控；
- production profile 可覆盖高频 error.type 为 optional/none，但 panic 必须有可用诊断出口；
- 如接入外部 Error Storage，只提供注入接口和 `app.error_id` 关联，不把对象存储依赖放入核心包；
- 不采用非规范 `exception.id`。

## 7. P2：性能、采样与后端接入

### RULE-P2-001：异步边界

- 核心 Logger 每次 Emit 同步完成归一化、采样、脱敏并调用 Writer，不内建 goroutine/定时器；
- OTLP 继续使用 SDK BatchProcessor，Collector 继续二次 batch；
- file Writer 默认保持可靠同步写；可选异步 Writer 必须配置有界队列、满队列策略、丢弃计数和 Shutdown flush；
- OpenSearch/ELK 通过 Collector exporter 或部署侧采集接入，不在核心新增直连客户端。

### RULE-P2-002：采样与性能证据

- 默认仍是未跳过 HTTP 请求全量 AccessEvent；
- 显式成功 access 采样必须说明它会放弃完整关联保证；
- 增加 Logger、file Writer、OTLP enqueue 的 benchmark 与 allocation 基线；
- 性能验收必须给出目标 QPS、事件大小、并发数和可接受丢弃策略，不能只写“低 CPU/低内存”。

## 8. 验收契约

| ID | 验收条件 | 测试层级 |
| --- | --- | --- |
| ACCEPT-P0-001 | 黑盒 12/12 条 file 事件都有 timestamp | blackbox |
| ACCEPT-P0-002 | file 每条事件可按 service/version/instance/environment 查询 | blackbox |
| ACCEPT-P0-003 | OTLP Service Metadata 只存在于 Resource，键符合 semconv 1.41.0 | in-memory OTLP |
| ACCEPT-P0-004 | EventName 首段与 Body/EventType 一致，业务拒绝不再出现 `business + error.*` | unit + blackbox |
| ACCEPT-P0-005 | error.type 低基数、app.error_code 精确，二者不互换 | unit + blackbox |
| ACCEPT-P0-006 | 每个未跳过 HTTP 请求恰好一个 AccessEvent，覆盖 2xx/4xx/5xx/panic | blackbox |
| ACCEPT-P0-007 | 同请求事件共享 trace/request；子 Span 可不同；后台事件无伪 request_id | blackbox |
| ACCEPT-P0-008 | 日志中不存在 parent_span_id，Trace Processor 能重建父子树 | integration |
| ACCEPT-P1-001 | 敏感键在嵌套 map/slice/group/LogValuer 中均被 Masker 处理 | unit + mutation |
| ACCEPT-P1-002 | stacktrace 超限时确定性截断且有截断标记 | unit + blackbox |
| ACCEPT-P2-001 | OTLP Shutdown 能 flush；队列满/Collector 不可用行为有可观测计数 | integration |
| ACCEPT-P2-002 | benchmark 报告包含 ns/op、B/op、allocs/op，不虚构固定性能目标 | benchmark |

查询验收示例：

```text
service.name="order-api" AND level="ERROR" AND timestamp >= now-1h
trace_id="<trace-id>"
app.order_id="ORD-1001"
service.version="1.2.3" AND error.type=*
```

具体 LogQL、OpenSearch DSL 和 Grafana 面板配置属于部署适配层，不写入核心事件模型。

## 9. 建议 PR 拆分

1. `fix(telemetry): 对齐 Resource semconv 并补齐事件时间`
   修正 Resource 键、file 双投影、Timestamp 与对应黑盒测试。
2. `refactor(log): 统一事件名与错误字段语义`
   引入 EventName/EventType 一致性、`app.error_code` 迁移、错误中间件 resolver、ADR 与迁移文档。
3. `feat(log): 增加上下文与有界堆栈治理`
   统一 Subject 提取、PII 策略、stack 截断/路径策略与安全测试。
4. `perf(log): 建立导出性能基线与可选异步 file writer`
   先提交 benchmark；只有明确队列和丢弃契约后才实现异步 file writer。

每个 PR 独立提交、独立验证，不把 P0/P1/P2 合成一次不可审阅的破坏性改动。

## 10. 待确认问题

- SPEC-LOG-001：file 投影是否要求 `service.version` / `service.instance.id` 在开发环境也必填，还是仅 production profile 必填？
- SPEC-LOG-002：`app.business_code` 是否在 v0.x 直接迁移为 `app.error_code`，还是保留一个发布周期的双写兼容？
- SPEC-LOG-003：业务拒绝的中间件默认名采用 `business.http.rejected`，是否符合期望？
- SPEC-LOG-004：生产环境 panic stack 是保留并截断，还是必须接入外部 Error Storage 后才允许移除？
- SPEC-LOG-005：异步 file Writer 的满队列策略选择阻塞、丢最新、丢最旧还是降级同步？
- SPEC-LOG-006：性能门槛的目标 QPS、平均事件大小与最大内存预算是多少？

这些问题不阻塞 P0 的 Timestamp 与 semconv Resource 键修复，但会阻塞错误字段破坏性迁移、
production stack 默认值和异步 file Writer 实现。
