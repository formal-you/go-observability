# 本地 issue 记录

本地用 Markdown 记录需求 / 缺陷 / 任务，模拟 GitHub Issue 的字段结构，不依赖远程仓库即可工作。每个文件是一次 issue 记录；内容稳定后，有远程仓库时按这些文件在 GitHub 新建 Issue，远程 Issue 是任务状态、讨论与完成证据的真源。

**每个 issue 对应一个独立分支：形成 issue 的同时开出 `feat/<slug>` / `fix/<slug>` 分支**，实现与合并都挂在该分支上。分支的实际创建时机遵循 [GitOps SOP](../gitops-sop.md#4-codex-授权语义) 的 Codex 授权语义（用户只说「创建 Issue」时不自动开分支，说「完成这个 Issue / 创建 PR」时才开分支）。

## 文件名约定

`NNNN-<slug>.md`：

- `NNNN` 为三位递增序号（`001`、`002` …），本目录内唯一；
- `<slug>` 为短横线连接的英文主题，规则见 [分支命名规范](../branching.md#33-slug-规则)。

示例：`023-observability-logging-hardening.md`。

## 模板

```markdown
# <标题>

- 状态：open | in-progress | closed
- 类型：feature | bug | refactor | chore | docs
- 分支：`feat/<slug>`（形成 issue 时同步开出；已同步到 GitHub 拿到编号 N 时按 [分支命名规范](../branching.md#32-issue-n--规则) 改为 `feat/issue-<N>-<slug>`）
- 关联 PR：`../pr/NNNN-<slug>.md`（实现后填写）

## 背景 / 为什么

<当前问题、触发原因、影响谁。说明不改会怎样；若是需求，写出用户故事或业务目标。>

## 目标 / 非目标

- 目标：<本 issue 要达成的结果>
- 非目标：<明确不在本次范围内的内容，避免范围蔓延>

## 影响范围

<涉及模块、文档、契约、配置、测试。>

## 验收标准

- [ ] <可验证、可执行、可度量的完成条件>
- [ ] <每条对应一个可观察结果或命令>

## 约束与风险

<兼容性、安全、性能、治理门禁、迁移影响。>

## 相关文档 / ADR

<列表：ADR、OpenAPI、模块指南、测试契约。>

## 验证方式

<实现后应跑的命令与预期。>
```

## 状态与类型

- 状态：`open` → `in-progress`（开出实现分支并开始实现时）→ `closed`（全部验收实现并有证据，或已标记 Superseded）。
- 类型：`feature` / `bug` / `refactor` / `chore` / `docs`。同步到 GitHub 时映射为互斥 `type:*` 标签：`feature` → `type:feature`，`bug` → `type:bug`，`refactor` / `chore` / `docs` → `type:task`；分类规则见 [GitHub 远程协作实操](../github-remote-workflow.md#issue-分类)。

## 与 GitHub Issue 的分工

| 载体 | 负责内容 |
| --- | --- |
| 本地 `docs/issues/NNNN-<slug>.md` | issue 的完整契约：背景、范围、验收、约束、验证方式；随代码版本演进 |
| GitHub Issue | 稳定后的编号、状态、标签、讨论、关联 PR 与完成证据真源 |
| `docs/adr/` | 已经决定且需要长期保留的架构选择及其理由 |
| `docs/pr/NNNN-<slug>.md` | 进行中的复杂实施规格；实现后回填到「关联 PR」 |
| `docs/history/` | 已完成、放弃或被取代的实施文档，不作为当前真源 |

## 生命周期

1. 新建 `NNNN-<slug>.md` 时按模板写全，同时形成 `feat/<slug>` / `fix/<slug>` 分支，实现与合并都挂在该分支上。
2. 内容稳定后同步到 GitHub 新建 Issue 并回填远程编号；未同步前保留本地文件。
3. 实施 PR 使用 `Refs #N` 或 `Closes #N` 关联远程 Issue（仅满足全部验收的最终 PR 用 `Closes #N`）。
4. 全部验收实现并有证据后置为 `closed`；仍然有效的契约保留在本地文件，一次性实施过程归档到 [docs/history/](../history/README.md)。
5. 需求被替代时在原文标记 `Superseded` 并链接替代 issue，不删除历史。

## 合格性要求

高风险、驱动变更与自动化的 GitOps 型 issue（配置迁移、依赖升级、安全 / 审计改造等）必须达到右列强度：

| 项目 | 普通 Issue | GitOps Issue |
| --- | --- | --- |
| 目的 | 报问题 / 提需求 | 声明变更、驱动自动化 |
| 风险评估 | 可选 | 必须 |
| 验收标准 | 可选 | 必须 |
| 回滚策略 | 不需要 | 必须 |
| 审计链路 | 不需要 | 必须 |
| 变更范围 | 可选 | 必须 |
| 结构化程度 | 中等 | 很高 |

回滚策略与审计链路写入模板的「约束与风险」节。
