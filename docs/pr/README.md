# PR 实施文档

本目录保存正在设计或实现、且仅靠 GitHub PR 正文不足以表达的实施规格。文件名与关联 issue 一一对应：`NNNN-<slug>.md`。普通小型 PR 不需要在仓库内重复创建文档。

## 文件名约定

`NNNN-<slug>.md`，编号与 slug 与关联 issue 完全一致（见 [本地 issue 记录](../issues/README.md)）；issue 模板的「关联 PR」字段在实现后回填为 `../pr/NNNN-<slug>.md`。

## 模板

```markdown
# <标题>

- 状态：draft | active | merged | abandoned | superseded
- 关联 Issue：`../issues/NNNN-<slug>.md`（远程编号存在时写 `Closes #N` / `Refs #N`）
- 分支：`<type>/[issue-<N>-]<slug>`
- PR 编号：<合并或关闭后填写>

## 变更原因

<问题、解决方式、用户可感知的行为变化。>

## 范围 / 非范围

<改动涉及的文件与包；明确不动的部分。>

## 验收条件

<与 issue 验收标准对应；环境阻塞明确标为 blocked，不勾选成通过。>

## 兼容性与安全

<破坏性变更与迁移方式，或确认无破坏性变更；示例、日志不含密钥与个人信息。>

## 验证结果

<实际运行的命令、输出与退出码摘要。>
```

## 生命周期

1. 开始实施前记录关联 Issue、范围、验收条件、兼容性和状态（Draft / Active）。
2. 实现期间以当前代码、正式黑盒测试和 Accepted ADR 为准，PR 文档不能反向覆盖这些真源。
3. PR 合并后，把仍有长期价值的决策提炼到 ADR 或当前指南，再将实施文档移动到 [`../history/`](../history/)。
4. PR 关闭但未合并时同样归档，状态标记为 Abandoned 或 Superseded，并说明原因或替代 PR。

归档文档至少记录 PR 编号、Issue、最终状态、合并或关闭日期。GitHub PR 本身继续保存 Review、CI 和提交讨论，仓库不复制完整会话。

Issue、分支、PR、CI、合并和关闭的完整流程以 [开发工作流](../workflow.md#issue--pr-开放流程) 为准。
