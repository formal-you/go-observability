# 贡献指南

欢迎提交 Issue 和 PR。参与社区即表示同意遵守 [贡献者公约](CODE_OF_CONDUCT.md)。

## 开发要求

- 使用 `go.mod` 声明的 Go 版本。
- 修改公共行为时同步测试、相关示例和 `CHANGELOG.md` 的 `[Unreleased]`。
- 领域专属 `business.*` 事件应留在接入方包；核心包只维护跨领域能力。
- 用户文档用中文为主，首次出现的 OpenTelemetry 术语可保留英文。

## 验证

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```

低内存环境可先运行前三项，并在 PR 中如实注明未执行的检查及原因。提交前检查 `git diff --check`，确保没有本机绝对路径、密钥或生成日志进入仓库。

## 提交与 PR

- 推荐 Conventional Commits，例如 `feat:`、`fix:`、`docs:`、`refactor:`。
- 一个提交聚焦一个可说明的变更；多批次工作可以提交多次，不必压成一个大提交。
- PR 应说明动机、用户可见变化、兼容性影响和验证结果。
- 破坏性变更必须给出迁移说明，并记录到 CHANGELOG。
- 安全问题遵循 [SECURITY.md](SECURITY.md)，不要提交公开复现。
