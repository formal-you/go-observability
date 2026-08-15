# 分支命名规范

本文件是 `formal-you/go-observability` 的分支命名真源。目标是在不引入 GitFlow 重模型的前提下，让分支名可读、可检索、可被 CI 校验，并与仓库的 Issue 驱动流程和 Conventional Commits 对齐。

## 1. 模型

- 默认分支只有 `main`。
- 所有工作都在短生命周期的非默认分支上进行，通过 PR 合入 `main`。
- 分支名不承载全部治理信息；Issue、PR 标题、labels 和 CI 是配套真源。
- 本仓库采用 squash merge，合并后旧分支不再作为开发基线。

## 2. 固定分支

| 分支 | 用途 | 规则 |
| --- | --- | --- |
| `main` | 唯一默认分支，保持可发布状态 | 禁止直接 push；只能通过 PR 合入 |

## 3. 开发分支

格式：

```text
<type>/[issue-<N>-]<slug>
```

示例：

```text
feat/issue-24-error-registry-contract
fix/issue-31-sync-local-main-remaining
docs/branch-naming-convention
chore/otel-v1.45-upgrade
```

### 3.1 `type` 取值

| type | 使用场景 |
| --- | --- |
| `feat` | 新功能、公共 API、能力增强 |
| `fix` | 缺陷、兼容性、文档回归修复 |
| `docs` | 纯文档、README、教程 |
| `refactor` | 不改变公共行为的结构调整 |
| `perf` | 性能优化 |
| `test` | 只改测试或验收 |
| `chore` | 维护、依赖升级、CI、同步、治理 |
| `build` | 构建系统或打包 |
| `ci` | CI 配置 |

`type` 必须与 PR 最终 squash commit 的 Conventional Commit 类型一致或同向。

### 3.2 `issue-N-` 规则

- 只要存在 GitHub Issue，必须写 `issue-<N>-`。
- 没有 Issue 且属于 `docs`、`chore` 等低风险维护，可省略该段。
- `feat`、`fix`、`refactor`、`perf`、`test` 涉及公共行为或跨文件改动时，原则上必须先建 Issue。

### 3.3 `slug` 规则

- 全小写 ASCII。
- 只允许 `a-z`、`0-9`、单个 `-`、单个 `.`。
- 不允许连续 `--`、不允许 `..`，不允许以 `-` 或 `.` 结尾。
- 建议控制在 60 字符内。
- 不把日期、作者名、机器名放入 slug；这些信息属于 commit message 或 tag。
- 使用短名词短语，例如 `telemetry-per-signal-output`，不使用 `my-fix-2026`。

## 4. 特殊分支

| 前缀 | 用途 | 约束 |
| --- | --- | --- |
| `release/v<semver>` | 发布准备 | 从 `main` 切出，合回 `main` 并打 tag |
| `prototype/<slug>` | 一次性原型或试验 | 可 rebase、可 force-push；不得直接合入 `main` |
| `archive/<slug>` | 归档历史快照 | 只读，不继续开发 |

以下前缀禁止推送到远程：

- `codex/`
- `backup/`
- `wip/`
- `tmp/`
- `agent/`
- `test/`（临时测试分支）

备份需求优先使用 tag 或 `archive/`，不使用 `backup/<date>`。

## 5. 命名校验

CI 会在 PR 打开时校验 head 分支名。规则允许：

```text
main
feat/issue-24-error-registry-contract
docs/branch-naming-convention
chore/otel-v1.45-upgrade
release/v1.0.0
prototype/b4-events          # 仅本地或原型用途，不应发起合入 main 的 PR
```

CI 对 `dependabot/*` 与 `gh-readonly-queue/*` 分支自动放行。

## 6. 分支改名

没有关联 PR 的本地分支：

```powershell
git branch -m <old-name> <new-name>
git push origin -u <new-name>
git push origin --delete <old-name>
```

有关联 PR 的分支，优先使用 GitHub 网页或 API 的 Rename branch，避免删旧推新导致 PR 关闭。

已合并到 `main` 的分支不再重命名，按清理策略归档或删除；只有用户明确要求时才操作。
