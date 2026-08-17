[CmdletBinding()]
param(
    [string]$Repo = "formal-you/go-observability",
    [string]$BaseBranch = "main",
    [ValidateSet("feat", "fix", "docs", "refactor", "perf", "test", "chore", "build", "ci")]
    [string]$Type = "refactor",
    [string]$Slug,
    [string]$Title,
    [string]$IssueBodyFile = ".issue-body.md",
    [string]$PrBodyFile = ".pr-body.md",
    [string]$Label = "type:task",
    [string[]]$Files = @(),
    [string]$FileList = "",
    [string]$CommitBody = "",
    [int]$WaitSeconds = 60,
    [switch]$SkipVerify,
    [switch]$NoMerge
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Fail($msg) { Write-Host "ERROR: $msg" -ForegroundColor Red; exit 1 }
function Info($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

# 0. 前置检查
if (-not (Get-Command gh -ErrorAction SilentlyContinue)) { Fail "未找到 gh CLI，请先安装并登录" }
if (-not (Test-Path -LiteralPath ".git")) { Fail "当前目录不是 git 仓库" }

$current = git branch --show-current
if ($current -ne $BaseBranch) { Fail "请先切换到 $BaseBranch 分支（当前是 $current）" }

$allFiles = @()
if ($Files) { $allFiles += $Files }
if ($FileList -and (Test-Path -LiteralPath $FileList)) {
    $allFiles += Get-Content -LiteralPath $FileList | Where-Object { $_ -and -not $_.TrimStart().StartsWith("#") } | ForEach-Object { $_.Trim() }
}
if (-not $allFiles) { Fail "必须用 -Files 或 -FileList 指定要提交的文件" }

if (-not (Test-Path -LiteralPath $IssueBodyFile)) { Fail "Issue body 文件不存在: $IssueBodyFile" }
if (-not (Test-Path -LiteralPath $PrBodyFile)) { Fail "PR body 文件不存在: $PrBodyFile" }

# 1. 创建 Issue
Info "创建 Issue"
$issueUrl = gh issue create --repo $Repo --title $Title --body-file $IssueBodyFile --label $Label
if ($LASTEXITCODE -ne 0) { Fail "gh issue create 失败" }
$issue = 0
if ($issueUrl -match "/(\d+)\s*$") { $issue = [int]$Matches[1] } else { Fail "无法从 '$issueUrl' 解析 Issue 编号" }
Info "Issue 编号: $issue"

# 2. 创建本地分支
$branch = "$Type/issue-$issue-$Slug"
Info "创建分支 $branch"
git switch -c $branch
if ($LASTEXITCODE -ne 0) { Fail "git switch 失败" }

# 3. 显式暂存文件
Info "暂存文件"
git add -- @allFiles
if ($LASTEXITCODE -ne 0) { Fail "git add 失败" }
$staged = git diff --cached --name-only
if (-not $staged) { Fail "暂存区为空，请检查 -Files / -FileList" }
Info "已暂存文件:`n$staged"

# 4. 本地验证（默认开启）
if (-not $SkipVerify) {
    Info "运行 gofmt / build / vet / test"
    $goChanged = @(git diff --cached --name-only --diff-filter=ACM | Where-Object { $_ -like "*.go" })
    if ($goChanged.Count -gt 0) { gofmt -w @goChanged }
    $env:GOMAXPROCS = "1"
    $env:GOGC = "30"
    go build ./...
    if ($LASTEXITCODE -ne 0) { Fail "go build 失败" }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { Fail "go vet 失败" }
    go test ./...
    if ($LASTEXITCODE -ne 0) { Fail "go test 失败" }
    git diff --cached --check
    if ($LASTEXITCODE -ne 0) { Fail "git diff --cached --check 失败" }
    if ($goChanged.Count -gt 0) { git add -- @goChanged }
}

# 5. 提交
Info "提交"
$commitArgs = @("-m", $Title)
if ($CommitBody) { $commitArgs += @("-m", $CommitBody) }
$commitArgs += @("-m", "Refs #$issue")
git commit @commitArgs
if ($LASTEXITCODE -ne 0) { Fail "git commit 失败" }

# 6. 推送
Info "推送分支"
git push -u origin $branch
if ($LASTEXITCODE -ne 0) { Fail "git push 失败" }

# 7. 创建 PR
Info "创建 PR"
$prBody = Get-Content -LiteralPath $PrBodyFile -Raw
if ($prBody -notmatch "Closes\s+#") {
    $prBody = "Closes #$issue`n`n" + $prBody
}
$verify = @"

## 验证结果

- go build ./... ✅
- go vet ./... ✅
- go test ./... ✅
- git diff --cached --check ✅
"@
$prBody = $prBody.TrimEnd() + "`n`n" + $verify
$tempPr = Join-Path $env:TEMP ("pr-body-$issue.md")
Set-Content -LiteralPath $tempPr -Value $prBody -NoNewline

$prUrl = gh pr create --repo $Repo --base $BaseBranch --head $branch --title $Title --body-file $tempPr
if ($LASTEXITCODE -ne 0) { Fail "gh pr create 失败" }
$pr = 0
if ($prUrl -match "/(\d+)\s*$") { $pr = [int]$Matches[1] } else { Fail "无法从 '$prUrl' 解析 PR 编号" }
Info "PR 编号: $pr"
Info "PR 地址: $prUrl"

if ($NoMerge) {
    Info "NoMerge 模式：流程到 PR 创建结束。"
    exit 0
}

# 8. 等待 CI 并合并
Info "初始等待 $WaitSeconds 秒后开始检查 CI"
Start-Sleep -Seconds $WaitSeconds
gh pr checks $pr --repo $Repo --watch
if ($LASTEXITCODE -ne 0) { Fail "gh pr checks 返回失败" }

for ($i = 0; $i -lt 5; $i++) {
    $state = gh pr view $pr --repo $Repo --json state --jq ".state"
    $mss = gh pr view $pr --repo $Repo --json mergeStateStatus --jq ".mergeStateStatus"
    Info "PR state=$state mergeStateStatus=$mss"
    if ($state -eq "MERGED") { Info "PR 已合并"; break }
    if ($mss -eq "BEHIND") {
        Info "PR 落后于 base，执行 update-branch"
        gh pr update-branch $pr --repo $Repo
        gh pr checks $pr --repo $Repo --watch
        continue
    }
    if ($mss -eq "CLEAN" -or $mss -eq "UNSTABLE") { break }
    Info "当前不可合并，20 秒后重试"
    Start-Sleep -Seconds 20
}

Info "合并 PR"
gh pr merge $pr --repo $Repo --squash
if ($LASTEXITCODE -ne 0) { Fail "gh pr merge 失败" }

# 9. 更新本地 main
Info "更新本地 $BaseBranch"
git fetch origin --prune
git switch $BaseBranch
git pull --ff-only
if ($LASTEXITCODE -ne 0) { Fail "git pull --ff-only 失败" }
Info "全部完成"