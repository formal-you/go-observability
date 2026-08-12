# 需求文档

本目录保存需要随代码版本演进的详细需求与验收契约。GitHub Issue 是任务状态、讨论和远程协作的真源；本目录不是第二套看板。

## 与 GitHub Issue 的分工

| 载体 | 负责内容 |
| --- | --- |
| GitHub Issue | Open / Closed 状态、负责人、标签、讨论、关联 PR 和完成证据 |
| `docs/issues/` | 复杂需求背景、范围与非范围、兼容性约束、可观察验收和测试要求 |
| `docs/adr/` | 已经决定且需要长期保留的架构选择及其理由 |
| `docs/pr/` | 进行中的复杂实施规格，不代替需求或 ADR |
| `docs/history/` | 已完成、放弃或被取代的实施文档，不作为当前真源 |

简单任务可以只使用 GitHub Issue。只有需求复杂、需要代码 Review 时与仓库版本一起阅读，或者验收契约需要长期追踪时，才在本目录创建文档。

## 生命周期

1. 创建文档时写明远程 Issue；尚未创建时临时标记 `Pending`，远程创建后立即回填编号和链接。
2. 文档描述用户问题和可观察结果，不用预设实现细节代替需求。
3. 实施 PR 使用 `Refs #N` 或 `Closes #N` 关联远程任务。
4. Issue 完成后保留仍然有效的需求契约；一次性实施过程归档到 `docs/history/`。
5. 需求被替代时在原文标记 `Superseded` 并链接替代 Issue，不删除历史。

## 当前需求

| 需求 | 远程 Issue | 状态 |
| --- | --- | --- |
| [统一 Writer 生命周期](writer-lifecycle.md) | [#19](https://github.com/formal-you/go-observability/issues/19) | Open |
