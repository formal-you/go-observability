# 贡献指南

欢迎 PR 与 Issue。参与即表示你同意遵守 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。

## 开发

1. 读 [AGENTS.md](AGENTS.md)（防漂移硬规则）。
2. Go **1.25+**（见 `go.mod`）。
3. 改代码后：

```bash
gofmt -w .
go vet ./...
go test ./...
# 可选
go test -race ./...
```

低内存环境可设 `GOMAXPROCS=1`、`GOGC=30`。

## 文档与配置样例

- 用户文档在 `docs/`；索引见 [docs/README.md](docs/README.md)。
- 配置样例字段注释：`example/config/`、`observability/templates/`。
- schema / 键名变更必须同步 README、相关 docs、example 与 CHANGELOG `[Unreleased]`。

## 提交

- Conventional Commits：`feat:` / `fix:` / `docs:` / `refactor!:` 等。
- 领域 `business.*` 事件不要加进核心 `types.go`（放接入方或 `example/mall`）。

## PR

- 附 `go test ./...` 通过说明。
- 破坏性变更在标题加 `!` 并写清迁移方式。
- 安全问题请走 [SECURITY.md](SECURITY.md)，不要开公开 Issue 贴利用细节。
