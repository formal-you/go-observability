# 开发流程体系（Development SOP）

> 状态：**体系搭建中（draft）**——文件集已就位，内容待逐项讨论定稿。
> 定位：让 Codex 高效、正确理解需求、给出更好意见、更快完成、验收完备、高质量交付的开发任务执行体系。
> 分工：本组 = 开发任务执行体系；[GitOps 治理](../gitops/README.md) = Issue/PR/合并/发布治理；两者互不替代。

## 体系目标

| 目标 | 落点 |
| --- | --- |
| 高效 | 上下文按需加载，只读当前阶段需要的真源 |
| 正确理解需求 | 需求收敛 → 契约草案 → 假设清单 → 需确认项 |
| 更好意见 | masterAgent 以架构师视角给选型与风险意见，用户拍板 |
| 更快完成 | 多 Agent 并行 + 任务卡明确边界 |
| 验收完备 | 黑盒先行 + DoD 清单 + 回归 |
| 高质量 | code-review 两轴评审闭环 |

## 文件路由

| 文档 | 何时读 |
| --- | --- |
| [dev-sop.md](dev-sop.md) | 主剧本：一个开发任务从下达到提交的 7 阶段流程 |
| [roles.md](roles.md) | 角色模型：masterAgent / subAgent 职责边界 |
| [requirement-clarify.md](requirement-clarify.md) | 模糊需求收敛：契约草案、假设清单、改动分级 |
| [acceptance.md](acceptance.md) | 契约与验收：黑盒先行、DoD、回归与漂移 |
| [testing.md](testing.md) | 正式测试契约：验证分级、黑盒验收与回归基线真源 |
| [agent-orchestration.md](agent-orchestration.md) | 多 Agent 编排：subAgent 任务卡、分工、评审闭环 |
| [templates/](templates/) | 可复用模板：任务卡、需求契约、验收清单 |

## 与 GitOps 的分工

| 文档 | 负责 | 不负责 |
| --- | --- | --- |
| 本组（dev-sop/） | 怎么正确开发一个任务（需求→验收→提交） | Issue/PR/合并/授权语义 |
| [GitOps 治理](../gitops/README.md) | Issue/PR/CI/合并/归档与授权语义 | 具体怎么开发一个任务 |

## 与 mattpocock-skills 的关系

技能按阶段按需触发（`triage` / `to-questionnaire` / `to-spec` / `tdd` / `implement` / `code-review` / `handoff` 等）；
本组文档负责定义「何时用、谁用、产出什么」，不替代插件技能本身。