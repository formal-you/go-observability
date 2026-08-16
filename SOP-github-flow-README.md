# SOP-github-flow.ps1 使用说明

脚本位置：仓库根目录 `SOP-github-flow.ps1`（`docs/workflow.md` 的「SOP 自动化脚本（参考）」有使用摘要）

作用：在一个命令里完成：

```text
创建 Issue -> 创建分支 -> 暂存文件 -> 本地验证 -> 提交 -> push -> 创建 PR -> 等 CI -> squash merge -> 更新本地 main
```

## 前提

- 已安装并登录 `gh`。
- 当前目录是目标 git 仓库根目录。
- 当前分支是 `main`（或 `-BaseBranch` 指定的分支）。
- 已准备好两个 body 文件，和一份要提交的文件清单。

## 每次只需准备 3 样东西

1. `.issue-body.md`：Issue 正文。
2. `.pr-body.md`：PR 正文。
3. 文件清单：可以用 `-Files` 参数，也可以写到一个 `.files.txt`，每行一个文件路径。

## 示例

### 方式一：直接传文件数组

```powershell
.\SOP-github-flow.ps1 `
  -Type refactor `
  -Slug newlogwriter-rename `
  -Title "refactor: rename Runtime.NewWriter to NewLogWriter and file to runtime_logwriter.go" `
  -Files @(
    "CHANGELOG.md",
    "telemetry/runtime_writer.go",
    "telemetry/runtime_logwriter.go"
  ) `
  -WaitSeconds 60
```

### 方式二：使用文件清单

先写 `.files.txt`，每行一个文件：

```powershell
$files = @'
CHANGELOG.md
README.md
telemetry/runtime_writer.go
telemetry/runtime_logwriter.go
'@
$files | Set-Content -LiteralPath .files.txt -NoNewline

.\SOP-github-flow.ps1 `
  -Type refactor `
  -Slug newlogwriter-rename `
  -Title "refactor: rename Runtime.NewWriter to NewLogWriter and file to runtime_logwriter.go" `
  -FileList .files.txt `
  -WaitSeconds 60
```

## 参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-Repo` | `formal-you/go-observability` | 目标仓库 |
| `-BaseBranch` | `main` | 基础分支 |
| `-Type` | `refactor` | 分支类型：`feat/fix/docs/refactor/perf/test/chore/build/ci` |
| `-Slug` | 必填 | 分支 slug，例如 `newlogwriter-rename` |
| `-Title` | 必填 | Issue/PR/commit 标题 |
| `-IssueBodyFile` | `.issue-body.md` | Issue 正文文件 |
| `-PrBodyFile` | `.pr-body.md` | PR 正文文件 |
| `-Label` | `type:task` | Issue 标签 |
| `-Files` | 空 | 要提交的文件数组 |
| `-FileList` | 空 | 文件清单路径，每行一个文件 |
| `-CommitBody` | 空 | 额外 commit body |
| `-WaitSeconds` | `60` | 创建 PR 后先等多少秒再查 CI |
| `-SkipVerify` | 关 | 跳过 `gofmt/build/vet/test` |
| `-NoMerge` | 关 | 只创建 PR，不等待 CI、不合并 |

## 重要规则

- Issue 编号由脚本自动解析，并用于分支名、`Refs #N`。
- PR 编号由脚本自动解析，并用于 `gh pr checks / merge`。
- 脚本会检查 `-Files` / `-FileList`，只会暂存你指定的文件，不会 `git add .`。
- `-WaitSeconds` 默认 60，符合“等 CI 至少 1 分钟”的要求。
- 如果 CI 通过但 PR 状态是 `BEHIND`，脚本会自动 `gh pr update-branch` 再等待。
- 如果 PR 状态不是 `CLEAN`，脚本不会强行合并，只会等待 5 轮后让 `gh pr merge` 自己失败。

## 临时文件

脚本会生成：

- `.issue-body.md` / `.pr-body.md`：你准备的内容。
- 临时 PR body 放在系统 `%TEMP%`，不会污染仓库。
- `.files.txt`：可选的文件清单。

这些文件不要 `git add`；脚本不会暂存它们。