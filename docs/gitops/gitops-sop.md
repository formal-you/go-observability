# GitOps SOP（开发工作流）

> 分工：开发任务从需求收敛到提交的执行剧本见 [开发流程体系](../dev-sop/README.md)；本文件是 GitOps 治理、提交与合并流程的真源。

## 变更步骤

1. 用 Issue 或变更说明写清用户问题、兼容性影响和验收条件。
2. 重大变更（公共 API、事件/错误/采样/安全/审计模型、包结构、Writer/OTel 映射或治理策略）
   先按 [ADR 约定](../adr/README.md) 在对应分类目录（errors / events / middleware /
   security-audit / telemetry 等）新增或更新 ADR 决策文档并同步 `docs/adr/README.md` 索引：
   状态 Proposed → 实现后 Accepted，ADR 与实现、黑盒测试、文档进入同一提交，
   不把决策理由只留在聊天或 PR 里。
3. 按 [正式测试契约](../dev-sop/testing.md) 先补独立黑盒测试，再做最小实现与白盒边界测试。
4. 更新受影响的 README、配置样例、CHANGELOG 与 ADR 索引。
5. 按 [正式测试契约](../dev-sop/testing.md) 的验证分级执行验证：文档类只做差异与链接检查；模块内做定向测试；公共行为/架构做黑盒与全量。
6. 按可独立说明的批次提交 Git commit；多批次工作应保留多次提交记录。

**全量门禁**（架构 / 全仓治理类变更或合并前兜底；文档与模块内改动按 [正式测试契约](../dev-sop/testing.md) 分级，不强制全量）：

```powershell
gofmt -w <modified-go-files>
$env:GOMAXPROCS=1
$env:GOGC=30
go build ./...
go vet ./...
go test ./...
git diff --check
```

资源允许时额外运行 `go test -race ./...`。`--help` 或仅 build 不能替代相关测试；全量测试按分级在架构类变更或合并前执行，文档 / 模块内任务不要求全量。

## Issue / PR 开放流程

本节是 Codex 处理 GitHub 任务的执行真源。技术需求与进度使用 GitHub Issue；需要随代码版本管理的复杂需求与验收契约写入 [`issues/`](issues/README.md)，但远程 Issue 仍是任务状态和讨论真源。跨工具流程待办才写入 [`todo.md`](../todo.md)，复杂实施规格可暂存于 [`pr/`](pr/README.md)。执行远程 Issue/PR、CI、squash merge、已合并分支重放或主分支同步时，读取 [GitHub 远程协作实操](github-remote-workflow.md)。

### 1. 建立 Issue

1. 先搜索同仓库的 Open / Closed Issue，确认不存在重复任务；重复需求补充到原 Issue，不新建副本。
2. 功能、缺陷、公共行为、兼容性或跨多个文件的工作必须有 Issue；单纯拼写修正可以直接在 PR 说明中记录。
3. Issue 至少写明问题、范围与非范围、可观察验收条件、兼容性/安全影响和验证方式。不能用预设实现细节替代用户问题。
4. 只有全部验收已实现并有证据时才关闭 Issue。多个 PR 共同完成一个 Issue 时，中间 PR 使用 `Refs #N`，最终 PR 才使用 `Closes #N`。
5. 本地 issue 记录在 `docs/gitops/issues/NNNN-<slug>.md`（模板见 [issues/README.md](issues/README.md)），内容稳定后同步为远程 Issue 并回填编号；复杂需求在本地文件保存详细契约，简单任务可只建远程 Issue，避免为每个任务复制一份本地文档。
6. 创建或更新 Issue 时设置一个 `type:*` 分类标签；本个人账号仓库用 Labels 模拟 Organization Issue Types，具体分类表和回退步骤见 [GitHub 远程协作实操](github-remote-workflow.md#issue-分类)。

### 2. 实施与创建 PR

1. 从最新默认分支创建非默认分支，按 [分支命名规范](branching.md) 使用 `<type>/[issue-<number>-]<slug>`；实施前确认工作区中的用户改动并将其排除在暂存范围外。
2. 先更新契约和独立黑盒测试，再实现、更新文档并按本文件门禁验证。每个提交只包含一个可说明的逻辑变化。
3. 未完成或仍需设计确认时创建 Draft PR；验收满足、本地门禁通过后再转为 Ready for review。
4. PR 正文必须包含关联 Issue、变更原因、用户可见行为、兼容性/安全影响和真实验证结果。环境阻塞必须明确标为 blocked，不能勾选成通过。
5. PR 创建后跟踪 required checks；失败时读取实际日志，在同一分支提交最小修复并重新验证。所有 required checks 通过前，不得宣称 PR 可合并。

### 3. 合并与完成

1. 合并前确认 PR 为 Ready、目标分支正确、required checks 全绿且没有未解决的 requested changes。本仓库默认使用 squash merge 保持线性历史。
2. 合并后核对 PR 状态为 Merged、最终提交位于默认分支；包含 `Closes #N` 的 Issue 应自动变为 Closed。未自动关闭时，先核对验收证据，再补充完成评论并关闭。
3. Issue 尚有未完成验收时保持 Open，创建后续 Issue 或保留清单；不能因为某个 PR 已合并就把部分完成写成全部完成。
4. 重大决策已在「变更步骤」按 ADR 约定落档，本步只负责把实施文档从 `docs/gitops/pr/` 移入 [`docs/history/`](history/README.md)，记录 PR、Issue、Merged / Abandoned / Superseded 状态和日期。
5. PR 关闭但未合并时，在 Issue 说明原因；有替代 PR 时标记 Superseded，没有替代实现时保持 Issue Open。远程/本地分支仅在用户明确要求清理后删除。

### 4. Codex 授权语义

| 用户指令 | Codex 可自动执行 |
| --- | --- |
| “创建/提交 Issue” | 查重、创建或更新 Issue，不创建代码分支 |
| “完成这个 Issue”“创建/提交 PR” | 建分支、实现、验证、提交、push、创建 PR、更新 Issue/PR、跟踪 CI |
| “合并 PR”“完成并合并” | required checks 通过后 squash merge，并核对 Issue 关闭与归档状态 |
| “关闭 Issue/PR” | 仅关闭明确完成、重复、无效或已被取代的目标，并记录原因 |

完成标准：Issue 验收有可复现证据；本地门禁和 required checks 均有明确结果；请求创建的 PR 已存在并正确关联 Issue；请求合并时 PR 已进入默认分支且 Issue 状态正确；`docs/gitops/pr/` 不遗留已完成实施文档。

## SOP 自动化脚本（参考）

本目录提供一键 GitHub 流程脚本 [`SOP-github-flow.ps1`](SOP-github-flow.ps1)（使用说明见
[`SOP-github-flow-README.md`](SOP-github-flow-README.md)），把本文「Issue / PR 开放流程」的机械步骤自动化：

```text
创建 Issue -> 创建分支 -> 暂存文件 -> 本地验证 -> 提交 -> push -> 创建 PR -> 等 CI -> squash merge -> 更新本地 main
```

### 每次准备 3 样东西

1. `.issue-body.md`：Issue 正文（模板见 [`template/.issue-body.md`](template/.issue-body.md)）；
2. `.pr-body.md`：PR 正文（模板见 [`template/.pr-body.md`](template/.pr-body.md)，脚本自动补 `Closes #N` 与验证结果段）；
3. 文件清单：`-Files` 数组或 `-FileList .files.txt`（每行一个文件）。

运行前把 body 模板从 `template/` 复制到仓库根目录，或给脚本传 `-IssueBodyFile` / `-PrBodyFile` 指向 `template/` 下的文件。

### 示例

```powershell
.\docs\gitops\SOP-github-flow.ps1 `
  -Type refactor `
  -Slug newlogwriter-rename `
  -Title "refactor: rename Runtime.NewWriter to NewLogWriter" `
  -Files @("CHANGELOG.md", "telemetry/runtime_logwriter.go") `
  -WaitSeconds 60
```

### 关键规则

- 只暂存显式指定的文件（`git add -- <paths>`），绝不 `git add .`；`.issue-body.md` / `.pr-body.md` / `.files.txt` 等临时文件不会被暂存。
- Issue / PR 编号自动解析，用于分支名、`Refs #N` / `Closes #N` 与后续 `gh pr checks` / `gh pr merge`。
- CI 通过但 PR 为 `BEHIND` 时自动 `gh pr update-branch` 再等待；非 `CLEAN` 不强行合并。
- 提交前默认运行 `gofmt` / `go build ./...` / `go vet ./...` / `go test ./...` / `git diff --cached --check`（`-SkipVerify` 可关闭）。
- 脚本不删除分支；合并后仍需按「合并与完成」核对 Issue 关闭与本地 main 同步。

完整参数表与详细用法见 [`SOP-github-flow-README.md`](SOP-github-flow-README.md)。

## 兼容性规则

- 事件名、字段键、枚举值和公共 Go API 都属于使用方契约。
- v0.x 可以发生破坏性变更，但必须在 CHANGELOG 标明并提供迁移说明。
- 核心包不接收特定业务领域的事件常量；领域扩展使用 `BusinessPayload.ExtraAttrs`。
- 文档、示例与测试必须只依赖仓库内公开资料，不引用个人电脑路径或未发布设计文档。

## 提交前检查

- 没有密钥、真实用户数据、绝对路径、临时 JSONL 或编辑器文件。
- README 示例可编译，命令注明运行目录并同时照顾 PowerShell 与 bash。
- 新增链接能解析到仓库内文件；外部服务尚未上线时明确写“发布准备中”。
- Writer、Provider 等资源在示例和测试中被正确关闭。

## Git 门禁

- 每次完成的变更都必须创建 commit；一次提交聚焦一个可说明的逻辑变更。
- 使用 `git add <explicit-paths>` 显式暂存本次文件，禁止 `git add .` / `git add -A`，避免混入用户改动。
- 提交信息使用 Conventional Commits（`feat:` / `fix:` / `docs:` / `refactor:`）。
- 不 push、不 force push，除非用户明确要求。
