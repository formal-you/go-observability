# GitHub 远程协作实操

本文供 Codex 在需要创建或更新 GitHub Issue、PR、CI、合并与主分支同步时选择性读取。通用生命周期和授权边界以 [GitOps SOP](gitops-sop.md#issue--pr-开放流程) 为准；本文只记录本仓库已验证的远程操作顺序、失败回退和收尾证据。

## 开始前

1. 运行 `git status -sb`，区分本次提交和用户已有改动；暂存始终使用显式路径。
2. 运行 `gh auth status` 和 `gh repo view --json nameWithOwner,defaultBranchRef`，确认账号、仓库和默认分支。
3. 运行 `git fetch origin main`，以 `origin/main` 而不是可能过期的本地 `main` 作为远程基线。
4. 搜索同主题 Issue 和当前 head 分支的 PR，复用已有目标，避免重复创建。

完成标准：工作区范围明确、GitHub 已认证、`origin/main` 已刷新、Issue/PR 不重复。

## Issue

- 功能、缺陷、公共行为或跨文件任务先创建 Issue；标题描述用户问题，正文包含范围、非范围、可观察验收、兼容性/安全和验证方式。
- 多个 PR 共同实施时使用 `Refs #N`；只有最后一个满足全部验收的 PR 使用 `Closes #N`。
- PR 合并不等于 Issue 必然完成。合并后读取 Issue 状态和剩余清单；未完成则保持 Open 并创建后续任务。
- 关闭重复、无效或被替代 Issue 时写明原因和替代链接，不只改变状态。

### Issue 分类

`formal-you/go-observability` 属于个人账号仓库，GitHub Organization Issue Types API
对其返回 `404`。本仓库使用互斥的 `type:*` Labels 作为分类真源：

| Label | 使用场景 | 与旧标签的关系 |
| --- | --- | --- |
| `type:feature` | 新能力、公共 API、架构增强或未来规划 | 可以同时保留 GitHub 默认 `enhancement` |
| `type:bug` | 可复现的实现、兼容性、文档或回归缺陷 | 可以同时保留 GitHub 默认 `bug` |
| `type:task` | 不改变产品行为的维护、治理、调研和基础设施任务 | 不强制添加 `enhancement` / `bug` |

创建或更新 Issue 时先读取当前 Labels，再执行以下分类：

1. 根据用户问题选择且只选择一个 `type:*` Label；无法确定时使用 `type:task`，并在 Issue 正文写明待确认边界。
2. 目标 Label 不存在时创建上述固定名称、说明和颜色，再应用到 Issue；不能因为 GitHub Issue Types 不可用而省略分类。
3. Issue 性质发生变化时移除旧的 `type:*` Label 后设置新值，保持互斥；`enhancement` / `bug` 只作为 GitHub 生态兼容标签，不是分类真源。
4. 仓库未来转移到 Organization 并启用原生 Issue Types 后，先制定迁移映射，再停止使用 `type:*`；迁移完成前不得维护两套互相冲突的分类状态。

完成标准：每个新建 Issue 恰有一个 `type:*` Label；Issue Forms 和远程 Issue 的标签一致；读取 Issue 即可判断其分类，不依赖标题前缀。

## 分支与已 Squash 历史

本仓库默认 squash merge。PR 合并后，原 feature 分支保留的是多提交历史，`origin/main` 只有一个新的 squash commit；继续在旧分支开发会显示双方分叉。直接 `git rebase origin/main` 可能重放已合并提交。

只保留合并后新增提交时：

```powershell
git fetch origin main
git log --oneline origin/main..HEAD
git rebase --onto origin/main <last-commit-already-in-merged-pr>
git rev-list --left-right --count origin/main...HEAD
```

最后一条应显示 `0 N`。更新已经存在的远端 feature 分支时使用 `git push --force-with-lease`；它只在远端仍是预期旧值时改写引用，不能使用无 lease 的强推。若无法明确 `<last-commit-already-in-merged-pr>`，停止重写，改从 `origin/main` 创建新分支并 cherry-pick 确认属于新任务的提交。

## 创建 PR

1. 运行本地门禁并确认差异只包含目标 Issue。
2. push 当前非默认分支；普通新分支使用 `git push -u origin <branch>`，重放后的既有分支使用 `--force-with-lease`。
3. 优先使用 GitHub connector 创建 PR；若 connector 因 token 权限返回 `403 Resource not accessible by personal access token`，保留已推送分支并回退到已认证的 `gh pr create`。
4. 未完成规格使用 Draft；验收和本地门禁已完成时创建 Ready PR。正文包含 Issue 关键字、影响、兼容性和真实验证结果。正文使用 GitHub 识别关键词 `Closes #N`（或 `Fixes #N` / `Resolves #N`）才能在合并时自动关闭 Issue；中文表述（如「解决 #N」）不会触发自动关闭。
5. 创建后运行 `gh pr view <N> --json baseRefName,headRefName,isDraft,state,statusCheckRollup,url`，核对目标分支和检查状态。

## CI 与审查

- required checks 以 GitHub 当前结果为准，本地通过不能替代远程门禁。
- 使用 `gh pr checks <N> --watch --interval 10` 等待检查；失败后读取失败 job 日志，修复根因并在同一分支提交。
- 环境阻塞与代码失败分开记录。不能把未运行或 blocked 的检查标记为 passed。
- 合并前确认 PR 为 Ready、required checks 全绿、没有未解决的 requested changes，且 base/head 与预期一致。

## Squash Merge 与主分支收尾

用户明确授权合并后执行：

```powershell
gh pr merge <N> --repo formal-you/go-observability --squash
gh pr view <N> --repo formal-you/go-observability --json state,mergedAt,mergeCommit,url
git fetch origin main
```

然后核对关联 Issue 状态：仅当 PR 正文使用了 `Closes #N` 等 GitHub 识别关键词时，合并才会自动关闭 Issue；否则需手动执行 `gh issue close <N> --repo formal-you/go-observability --reason completed`，并用 `gh issue view <N> --json state,stateReason` 验证 stateReason=COMPLETED。更新本地 `main` 前先确认它没有独有提交：

```powershell
git switch main
git rev-list --left-right --count main...origin/main
git merge --ff-only origin/main
git status -sb
```

如果结果显示本地 `main` 对 `origin/main` 有 ahead 提交，停止自动更新，不能 reset 或覆盖；先向用户报告本地独有提交。只有 `0 N` 才允许 fast-forward。切换后 `git status -sb` 应显示 `main...origin/main` 且无工作区改动。

分支删除是单独的清理操作；合并和切换 `main` 不隐含删除本地或远端 feature 分支。

## 完成证据

最终回复至少给出：Issue URL 与状态、PR URL 与 Merged 状态、squash commit、本地/远端验证结果、当前分支和工作区状态，以及未执行或受环境阻塞的门禁。
