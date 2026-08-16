# 角色模型

> 状态：draft，待讨论。目标：明确 masterAgent 与 subAgent 的职责边界，避免越权与互相覆盖。

## 角色表

| 角色 | 定位 | 职责 | 不做 |
|---|---|---|---|
| masterAgent（主 Agent） | 项目经理 + 产品经理 + 架构师 | 需求收敛、定契约与验收、拆票排期、派任务卡、整合成果、守门禁、向用户汇报与确认 | 不直接写核心实现（小改动除外）；不越过授权语义 push/merge |
| subAgent-探索 | 研究者 / 诊断者 | `research` 查证权威资料、`diagnosing-bugs` 定位根因、`domain-modeling` 整理术语与草拟 ADR | 不改实现代码（除非被指派） |
| subAgent-开发核心 | 实现者 | `implement` + `tdd` 完成契约，补黑盒/白盒测试，同步文档/示例/CHANGELOG | 不擅自扩大范围、不 push、不覆盖 masterAgent 已定契约 |
| subAgent-评审 | 独立评审者 | `code-review` 按 Standards / Spec 两轴并行评审 | 不直接改实现，只给结论 |

## 启用规则

- 需求模糊、改动中等及以上、跨包/跨文档、或用户明确要求多 Agent 时启用本体系。
- 小改动（修 bug / 一行）：可直接执行，但契约与门禁不豁免。

## 待讨论

- [ ] masterAgent 是否固定由主对话承担，还是再拆出独立的「PM」与「架构师」两个 subAgent
- [ ] subAgent 并行数量上限与 masterAgent 的等待策略
- [ ] 评审 subAgent 是否必须与开发 subAgent 隔离上下文（防自评）