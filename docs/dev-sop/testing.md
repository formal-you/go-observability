# 正式测试契约

> 状态：**重写版（v2，draft 待评审）**。本文定义「什么变更需要什么验证」：按任务类型分级，不再一刀切要求全量测试。
> 目标：验收随任务走——文档更新不跑代码测试，模块更新只跑定向测试，公共行为 / 架构才需要黑盒与全量。

## 0. 核心原则

1. **验收随任务走**：不同任务（类型、范围、所在分支）有不同的验收标准；同一任务的验收标准在其 issue / 分支上维护（动态验收清单），不是仓库级一刀切。
2. **黑盒优先（不变）**：公共行为、契约、兼容性变化，必须先有独立黑盒测试再实现；实现不得反向定义预期。
3. **验证分级**：按任务类型决定跑什么（§3）；「全量测试」是架构 / 全仓治理类或合并前 CI 的兜底，不是每次改动的默认。

## 1. 权威顺序（不变）

1. 用户已确认的需求、[CONTEXT.md](../../CONTEXT.md) 与已接受 ADR 定义预期。
2. 独立黑盒测试把预期变成可重复执行的正式验收。
3. 实现代码与白盒单元测试证明内部实现，但不能反向修改需求预期。

当三者冲突时，先确认或更新契约，再更新黑盒 Oracle，最后修改实现。禁止仅为让现有代码变绿而降低黑盒断言。

## 2. 什么是有效黑盒测试（不变）

有效黑盒测试只通过公开 API、真实协议入口或最终输出观察行为，不读取未导出的实现细节：

- `log/blackbox_log_test.go`、`log/event_keep_sampler_blackbox_test.go` 使用外部测试包验证公开日志契约。
- `example/blackbox/blackbox_test.go` 发起真实 Gin 请求、创建真实 OTel span，并验证 JSONL 与内存 OTLP 输出。
- Writer 黑盒按最终 JSONL / LogRecord 形状断言，不复制内部归一化算法。
- 中间件黑盒按请求、响应、事件数量和关联字段断言，不以内部调用顺序充当 Oracle。

外部 Collector、Loki、Tempo 等不可控系统可以使用内存 exporter 或明确的集成测试边界替代；请求生命周期、trace/span 生成、序列化和错误投影应尽量走真实代码路径。

## 3. 验证分级矩阵

| 任务类型 | 典型场景 | 验收方式 | 最小验证命令 | 是否跑 go test |
| --- | --- | --- | --- | --- |
| 文档 | README / docs / 注释 / 示例文案 | 链接可解析、无残留旧路径、diff 干净 | `git diff --check` + 链接检查 | **否** |
| 模块内改动 | 只动单个包内部实现，公共 API / schema 不变 | 该包定向单测 + 受影响黑盒（如已有） | `go test ./<pkg>...` | 定向即可，不必全量 |
| 跨包 / 公共行为 | 公共 API、Writer、中间件、schema、事件/错误模型 | 独立黑盒先行（红→绿）+ 受影响包白盒 | 黑盒 + `go test ./<affected>...` | 受影响包 + 相关黑盒 |
| 架构 / 全仓治理 | 重构、依赖升级、CI、跨多包治理 | 黑盒回归 + 全量门禁 | `gofmt` / `go build ./...` / `go vet ./...` / `go test ./...` / `git diff --check` | 全量（资源允许加 `-race`） |

- **文档更新不需要代码测试**：只做 `git diff --check`、链接与残留路径检查；涉及示例代码时确认示例可编译。
- **模块更新不要求全量** `go test ./...`：跑受影响包（含其依赖方向）的定向测试即可；全量测试由合并前 CI 兜底。
- `--help` 或仅 build 不能替代相关测试；但「相关」按任务类型界定。

## 4. 动态验收与回归

- 核心验收随需求迭代更像回归测试：需求变化时先更新契约与黑盒，再动实现；黑盒是跨任务的回归基线。
- **任务不同、所在 git 分支不同，验收标准可以不同**：每个任务在 `docs/gitops/issues/NNNN-<slug>.md` 维护自己的验收清单；「跑哪些测试」按该任务类型（§3）决定，不按「每次全量」。
- 文档漂移防护：新增决策必须进 ADR + 索引 + 路由表同步，避免真源分裂。

## 5. 变更流程（分级版）

1. 先判断任务类型（文档 / 模块内 / 跨包公共行为 / 架构全仓），确定验收方式与验证命令（§3）。
2. 公共行为变更：先新增或调整独立黑盒测试，并确认它因缺失目标行为而失败（红）。
3. 完成最小实现，再补白盒单元测试覆盖边界、错误分支和内部不变量（绿）。
4. 按 §3 分级跑验证命令；文档类不跑 go test，模块内不强制全量。
5. 契约、黑盒、实现、示例和迁移说明在同一逻辑提交中保持一致。

黑盒测试可以在明确的契约变更中更新，但提交必须同时给出需求或 ADR 依据；纯重构不得改变黑盒期望。

## 6. 当前核心验收（回归基线）

涉及以下契约的任务，改动后必须让对应黑盒 / 断言回归通过：

- EventName 文法、ErrorType / ErrorCode 分工以 [ADR-0009](../adr/events/0009-event-name-fact-and-error-code.md) 为准。
- event.name 的 `<event>` 必须是注册 Event Type（Event Name Convention）以 [ADR-0018](../adr/events/0018-event-name-convention.md) 为准。
- ErrorCode / ErrorType 严格文法、构造失败和旧入口兼容以 [ADR-0010](../adr/errors/0010-strict-error-construction.md) 为准。
- ErrorType 复用 OTel/gRPC 标准枚举（跨模块闭合枚举）以 [ADR-0016](../adr/errors/0016-errortype-otel-grpc-standard-enum.md) 为准。
- ErrorCode → ErrorType 固定映射（Error Registry）以 [ADR-0015](../adr/errors/0015-error-registry.md) 为准，严格构造器对已注册码强制类型一致。
- file/stdout 与 OTLP 双投影以 [OTel Logs 映射](../reference/otel-logs-data-model.md) 为准。
- HTTP AccessEvent 完整性、跨事件关联和后台事件边界以 [`example/blackbox`](../../example/blackbox/README.md) 的公开场景与黑盒断言为准；最初的实施计划已归档为 [PR #16 历史提案](../gitops/history/pr-0016-logging-system-optimization.md)。
- 采样只在使用方显式配置后改变保留行为；默认行为必须由黑盒覆盖。