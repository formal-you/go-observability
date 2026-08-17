# 历史文档

本目录保存已经完成、放弃或被取代的实施计划和阶段材料。它们用于追溯当时的背景与取舍，不是当前配置、接口或测试行为的真源。

阅读优先级：当前指南与代码契约 > 状态为 Accepted 的 ADR > 历史文档。归档文件必须保留最终状态、关联 Issue/PR 和日期，并在内容已知漂移时明确提示。

Accepted ADR 不移动到这里；ADR 即使已经实现，仍负责解释当前决策。只有 ADR 状态变为 Deprecated 或 Superseded 时，才在 ADR 原目录标注替代关系，编号和文件仍保留。

## 已归档

| 文档 | 最终状态 | 实现依据 |
| --- | --- | --- |
| [PR #16 日志系统优化提案](pr-0016-logging-system-optimization.md) | Merged，2026-08-11 | [Issue #14](https://github.com/formal-you/go-observability/issues/14)、[PR #16](https://github.com/formal-you/go-observability/pull/16)、[验收报告](../../reports/issue-14-acceptance.md) |

进行中的复杂 PR 规格放在 [`../pr/`](../pr/)；可复现的性能、mutation 和验收快照继续放在 [`../../reports/`](../../reports/README.md)。
