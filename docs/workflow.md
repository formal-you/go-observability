# 开发流程与协作规则（Workflow）

> 面向开始干活的新人：一个需求/决策怎么从想法走到代码；以及本机的硬约束。

## 全流程（设计决策型）

```
wayfinder（读 MAP → 认领一张 frontier 票）
  → grill-with-docs（HITL 决策轮：❓Q + ➡️推荐，用户判定）
  → prototype（throwaway 分支验证方案，归档不进 main）
  → implement / tdd（代码落地，测试先行）
  → code-review（双轴：规范 + 规格）
  → handoff（更新 SESSION-HANDOFF，交接下个会话）
```

- 一次会话一张决策票；判定后写 Resolution → close → MAP Decisions-so-far 追加。
- **轻量黑盒防幻觉层**：确定性规则的测试用**外部包黑盒用例**（`package log_test` / `telemetry_test`），
  期望值来自 `../../observability-design/spec/acceptance.md` 的 Oracle（决策表/文法），不读实现内部——
  避免"同代理写实现+测试"的自证幻觉。黑盒用例引用 RULE/ACCEPT/CASE ID 追踪。

## git 纪律（2026-08-09 用户规范：每次改动必须 commit）

- 每次改动后必须 commit；暂存用显式路径 `git add <path>`，**禁止** `git add -A` / `git add .`。
- 提交信息用 conventional commits（feat/fix/docs/test: 简述）。
- 不 push、不 force push，除非用户明确要求。

## 本机约束

- 低内存组合：`$env:GOMAXPROCS=1; $env:GOGC=30` 再 `go build/vet/test ./...`。
- 本机无 docker：`../observability/` compose/面板只做 YAML/JSON 静态校验，不跑容器。
- 长 PowerShell 命令拆小；含 `$` 的整块命令用单引号 here-string。
- 改 Go 代码后必须 `gofmt -w` + 全量 test。

## 改代码前必读

`../AGENTS.md` 的防漂移硬规则（摘要，原文为准）：

1. 事件名必须用 `types.go` 常量注册表，禁止手写事件名字符串。
2. semconv 键以 1.41.0 为准（如 `url.path`、`code.function.name`）。
3. LogRecord 顶层字段映射集中在 `internal/attrkv.Record`。
4. 新增/修改字段必须在 `keys.go` 登记；vendor 键一律 `app.*`。
5. 双投影：file/stdout 保留顶层键，OTLP 剥离；不要为 OTLP 塞回属性、为 file 拆掉属性。
6. schema/键名/字段归属变更必须同步文档（README / detailed-design / 方案2）。
7. 零值省略：字符串/数值零值省略，布尔不省略。
8. samber 生态只允许出现在 `example/` 与 `docs/samber-comparison.md`。

## 文档同步义务

改代码 ≠ 只改代码：事件名/键名/字段归属变了，同步
`../../observability-design/outline/B1-event-structs/detailed-design.md`、`方案2-直接符合规范.md`、`README.md`；
术语变了回填 `CONTEXT.md`；决策变了写 `DESIGN-DECISIONS.md` + wayfinder MAP。
