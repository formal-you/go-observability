# 开发任务 SOP（多 Agent 编排）

> 状态：**demo 草案**——供评审迭代，确认后转正式。
> 真源分工：本文是「一个开发任务从需求下达到提交」的执行剧本；Issue/PR/合并与授权语义以 [GitOps SOP](../gitops/gitops-sop.md) 为准；术语以 [CONTEXT.md](../../CONTEXT.md) 为准；黑盒验收契约以 [testing.md](testing.md) 为准。
> 技能来源：引用的技能来自 mattpocock-skills 插件（mattpocock-skills:<skill>），按需触发，不要求全部加载。
> 组索引：[开发流程体系](README.md)（角色 / 需求收敛 / 验收 / 多 Agent 编排 / 模板）。

## 1. 角色模型

| 角色 | 定位 | 职责 | 不做 |
|---|---|---|---|
| masterAgent（主 Agent） | 项目经理 + 产品经理 + 架构师 | 需求收敛、定契约与验收、拆票排期、给 subAgent 派任务卡、整合成果、守门禁、向用户汇报与确认 | 不直接写核心实现（小改动除外）；不越过 [GitOps SOP](../gitops/gitops-sop.md) 授权语义做 push/merge |
| subAgent-探索 | 研究者 / 诊断者 | `research` 查证权威资料、`diagnosing-bugs` 定位根因、`domain-modeling` 整理术语与草拟 ADR | 不改实现代码（除非被指派） |
| subAgent-开发核心 | 实现者 | `implement` + `tdd` 完成契约，补黑盒/白盒测试，同步文档/示例/CHANGELOG | 不擅自扩大范围、不 push、不覆盖 masterAgent 已定契约 |
| subAgent-评审 | 独立评审者 | `code-review` 按 Standards / Spec 两轴并行评审 | 不直接改实现，只给结论 |

**何时启用**：需求模糊、改动中等及以上、跨包/跨文档，或用户明确要求多 Agent 时。单行修复、纯拼写等小改动可直接执行，但契约与门禁不豁免。

## 2. 执行剧本

| 阶段 | 主持人 | 技能（mattpocock-skills） | 产出 / 门禁 |
|---|---|---|---|
| 0 需求澄清与收敛 | masterAgent | `triage` / `to-questionnaire` / `grill-with-docs` / `wait-what` | 契约草案（问题/范围非范围/验收/影响）、假设清单、需用户确认项；写入 `docs/gitops/issues/NNNN-<slug>.md` |
| 1 探索与研究 | subAgent-探索 | `research` / `diagnosing-bugs` / `domain-modeling` | 调研结论 / 根因分析 / 术语卡片；重大变更草拟 ADR（Proposed） |
| 2 设计决策 | masterAgent（架构师） | `codebase-design` / `prototype` / `domain-modeling` | 选型结论或 ADR；需用户拍板的项列给用户，不自行决定 |
| 3 契约先行 | masterAgent + subAgent-开发核心 | `to-spec` / `to-tickets` / `tdd`（红） | spec + tracer-bullet tickets + 独立黑盒测试先红（仓库硬门禁） |
| 4 开发核心 | subAgent-开发核心 | `implement` / `tdd`（红-绿-重构）/ `diagnosing-bugs` | 最小实现 + 白盒边界测试（绿） |
| 5 文档同步 | subAgent-开发核心 | `writing-for-agents`（涉及 AGENTS.md 时）/ 手动同步 README/示例/CHANGELOG/ADR 索引 | 单一真源：schema/API 变化与文档、示例、测试同提交 |
| 6 验证与评审 | masterAgent + subAgent-评审 | `code-review` / 全量门禁命令 | 两轴评审结论 + gofmt / build / vet / test / race / diff --check 全绿 |
| 7 收尾提交 | masterAgent | `handoff`（跨会话时） | 更新 issue 状态；显式路径暂存；Conventional Commit；按 [GitOps SOP](../gitops/gitops-sop.md) 授权语义 push/PR/merge |

## 3. 缝合协议（每阶段开始，降幻觉）

1. 重述任务与验收（一句话）；
2. 按 [AGENTS.md](../../AGENTS.md) 路由读取对应真源，不凭记忆或摘要；
3. 冲突检查：与 CONTEXT.md / Accepted ADR / CHANGELOG / 既有黑盒断言冲突时，先契约后代码；
4. 列出假设与需确认项，模糊点不猜测；
5. 先契约/黑盒，再实现；
6. 跑真实验证命令，`--help` 或仅 build 不算通过。

## 4. subAgent 任务卡模板

```text
- 任务名 / 目标（一句话）
- 范围与禁止项（文件、包、不碰什么）
- 真源指针（必须读的文档）
- 输入（issue / PRD / spec / tickets）
- 输出与验收（文件路径 + 黑盒断言 + 验证命令）
- 提交规则（是否允许 commit / push）
- 完成后 masterAgent 评审点（spec 符合性 → 代码质量）
```

## 5. 完成定义（DoD）

- [ ] 契约与黑盒测试先于实现且为绿
- [ ] 文档 / 示例 / CHANGELOG / ADR 索引同步（单一真源）
- [ ] 全量门禁通过：`gofmt` / `go build ./...` / `go vet ./...` / `go test ./...` / `git diff --check`（资源允许加 `go test -race ./...`）
- [ ] 工作区中的用户改动未被混入本次提交
- [ ] 显式路径暂存 + Conventional Commit；不 push（除非用户明确要求）

## 6. 与 GitOps SOP 的分工

| 文档 | 负责 | 不负责 |
|---|---|---|
| [dev-sop.md](dev-sop.md)（本文） | 任务执行剧本：需求收敛、多 Agent 编排、契约先行、验证、DoD | Issue/PR/合并/授权语义 |
| [gitops-sop.md](../gitops/gitops-sop.md) | Issue/PR/CI/合并/归档的 GitOps 治理与授权语义 | 具体怎么开发一个任务 |