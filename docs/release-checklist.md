# 发布检查清单

首次公开发布前必须逐项完成；任一安全门禁未满足都应阻断发布。

## 仓库与安全

- [x] 创建公开 GitHub 仓库，并确认远程仓库 URL、仓库所有者和 `go.mod` module path 完全一致。
- [x] 配置本地 `origin`，推送默认分支，并从未登录窗口确认 README、LICENSE、CI 和仓库链接可访问。
- [ ] 在 **Settings > Security > Code security and analysis** 启用 Private vulnerability reporting。
- [ ] 在 **Settings > Branches** 或 Rulesets 为默认分支启用 branch protection：要求 PR、CI 通过和禁止强推。
- [ ] 确认 SECURITY.md 的私密报告路径实际可用，不发布虚构邮箱。

## 版本质量

- [ ] `gofmt`、`go vet ./...`、`go test ./...`、`go test -race ./...`、`govulncheck ./...` 全部通过。
- [ ] README 最小示例可编译并生成预期 JSONL；Gin、net/http、Metric、OTLP 示例可运行。
- [ ] 清理本机路径、内部决策术语、失效 badge、临时日志和敏感数据。
- [ ] 将 CHANGELOG 的 `[Unreleased]` 内容整理到 `v0.1.0`，补发布日期与比较链接。

## 发布与外部验证

- [ ] 创建并推送带注释标签：`git tag -a v0.1.0 -m "v0.1.0"`，然后 `git push origin v0.1.0`。
- [ ] 在一个全新的临时模块中执行 `go get github.com/formal-you/go-observability@v0.1.0`，编译 README 示例。
- [ ] 打开对应 pkg.go.dev 页面，确认根包、`telemetry`、writer 和 middleware 文档可索引。
- [ ] 从仓库外网络检查 GitHub release、tag、CI badge、源码链接和文档相对链接。
- [ ] 只有以上检查全部通过后，才在 README 展示安装命令、CI badge、Go Reference badge 和稳定支持声明。
