# 多 Agent 编排

> 状态：draft，待讨论。目标：subAgent 负责探索与开发，masterAgent 负责编排、整合与守门禁。

## 任务卡

每个 subAgent 必须拿到任务卡（模板见 [templates/subagent-task-card.md](templates/subagent-task-card.md)）：
目标、范围/禁止项、真源指针、输入、输出与验收、验证命令、提交规则、评审点。

## 分工原则

- 写面不重叠：调研 / 黑盒测试 / 实现 / 文档同步 / 评审 各自独立文件集；
- 可并行：独立探索、调研、黑盒测试编写可并行；实现依赖契约先行；
- 串行门禁：契约 → 实现 → 验证 → 评审，前序不绿不进下一步。

## 评审闭环（subAgent-评审 / code-review）

1. spec 符合性：是否满足 issue / 契约；
2. 代码质量：是否符合 `docs/coding-standards.md`；
3. 结论交 masterAgent，必要时回退开发核心修复。

## 上下文缝合（降幻觉）

每阶段开始：重述任务 → 读真源 → 冲突检查 → 列假设 → 契约先行 → 真实验证命令（`--help` 或仅 build 不算通过）。

## 待讨论

- [ ] subAgent 会话隔离与上下文传递（`handoff`）
- [ ] 并行上限与 masterAgent 等待策略
- [ ] 与 mattpocock-skills 的触发映射是否直接引用插件技能名