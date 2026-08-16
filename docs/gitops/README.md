# GitOps 治理组

本组收纳仓库的 GitOps 治理文档：从分支命名、Issue/PR 记录与归档，到远程实操、发布清单与总流程。开发任务的具体执行剧本不在这里，见 [开发任务 SOP](../dev-sop.md)。

## 入口

| 文档 | 何时读 |
| --- | --- |
| [GitOps SOP](gitops-sop.md) | 任何代码/文档变更的总流程与门禁：验证、文档同步、提交、授权语义 |
| [分支命名规范](branching.md) | 创建、重命名分支或调整分支治理 |
| [本地 issue 记录](issues/README.md) | 写需求/缺陷契约、同步 GitHub Issue |
| [PR 实施文档](pr/README.md) | 复杂 PR 的实施规格与归档规则 |
| [历史文档](history/README.md) | 追溯已完成/放弃/被取代的实施文档（非当前真源） |
| [GitHub 远程协作实操](github-remote-workflow.md) | 创建/更新 Issue/PR、处理 CI、squash merge 与同步主分支 |
| [发布检查清单](release-checklist.md) | 首次公开发布和后续发版 |

## 与开发任务 SOP 的分工

| 文档 | 负责 | 不负责 |
| --- | --- | --- |
| [开发任务 SOP](../dev-sop.md) | 任务执行剧本：需求收敛、多 Agent 编排、契约先行、验证、DoD | Issue/PR/合并/授权语义 |
| [GitOps SOP](gitops-sop.md) | Issue/PR/CI/合并/归档的 GitOps 治理与授权语义 | 具体怎么开发一个任务 |